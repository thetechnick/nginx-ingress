package controller

import (
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/thetechnick/nginx-ingress/pkg/collision"
	"github.com/thetechnick/nginx-ingress/pkg/config"
	"github.com/thetechnick/nginx-ingress/pkg/parser"
	"github.com/thetechnick/nginx-ingress/pkg/storage"
	"github.com/thetechnick/nginx-ingress/pkg/storage/pb"
	api_v1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/record"
)

// Configurator converts ingress objects into nginx config
type Configurator interface {
	ConfigUpdated(cfgm *api_v1.ConfigMap) error
	IngressDeleted(ingKey string) error
	IngressUpdated(ingKey string) error
}

// NewConfigurator creates a new Configurator instance
func NewConfigurator(
	ingExStore IngressExStore,
	recorder record.EventRecorder,
	mcs storage.MainConfigStorage,
	scs storage.ServerConfigStorage,
) Configurator {
	return &configurator{
		scs:             scs,
		mcs:             mcs,
		ingExParser:     parser.NewIngressExParser(),
		configMapParser: parser.NewConfigMapParser(),
		ch:              collision.NewMergingCollisionHandler(),
		ingExStore:      ingExStore,
		configurator:    NewRenderer(),
		recorder:        recorder,
		log:             log.WithField("module", "Configurator"),
	}
}

type configurator struct {
	mainConfig      *config.Config
	mcs             storage.MainConfigStorage
	scs             storage.ServerConfigStorage
	ingExParser     parser.IngressExParser
	configMapParser parser.ConfigMapParser
	ch              collision.Handler
	ingExStore      IngressExStore
	configurator    Renderer
	mutex           sync.Mutex
	recorder        record.EventRecorder
	log             *log.Entry
}

func (c *configurator) ConfigUpdated(cfgm *api_v1.ConfigMap) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	nginxConfig, err := c.configMapParser.Parse(cfgm)
	if err != nil {
		if verr, ok := err.(*parser.ValidationError); ok {
			for _, e := range verr.Errors {
				c.recorder.Event(cfgm, api_v1.EventTypeWarning, "Config Error", e.Error())
			}
		} else {
			return err
		}
	}
	c.mainConfig = nginxConfig

	configUpdate, err := c.configurator.RenderMainConfig(nginxConfig)
	if err != nil {
		return err
	}
	return c.mcs.Put(configUpdate)
}

func (c *configurator) IngressDeleted(deletedIngressKey string) error {
	return c.IngressUpdated(deletedIngressKey)
}

// involvedIngressObjects returns the ingress objects needed to update the given server configs
func involvedIngressObjects(serverConfigs []*pb.ServerConfig) map[string]bool {
	ingressObjects := map[string]bool{}
	for _, serverConfig := range serverConfigs {
		for ingressKey := range serverConfig.Meta {
			ingressObjects[ingressKey] = true
		}
	}
	return ingressObjects
}

func (c *configurator) parseServerConfig(ingEx *config.IngressEx) ([]*config.Server, error) {
	servers, err := c.ingExParser.Parse(*c.mainConfig, ingEx)
	if err != nil {
		if verr, ok := err.(*parser.IngressExValidationError); ok {
			c.handleIngressExValidationError(verr)
		} else {
			return nil, err
		}
	}
	return servers, nil
}

func mapFromServerList(servers []*pb.ServerConfig) map[string]*pb.ServerConfig {
	serverMap := map[string]*pb.ServerConfig{}
	for _, server := range servers {
		serverMap[server.Name] = server
	}
	return serverMap
}

func (c *configurator) dependencyMap(ingressKey string, serverNames []string, dependencies map[string]map[string]*pb.ServerConfig) (map[string]map[string]*pb.ServerConfig, error) {
	if dependencies == nil {
		dependencies = map[string]map[string]*pb.ServerConfig{}
	}

	servers, err := c.scs.ByIngressKey(ingressKey)
	if err != nil {
		return nil, err
	}
	for _, serverName := range serverNames {
		s, err := c.scs.Get(serverName)
		if err != nil {
			return nil, err
		}
		if s != nil {
			servers = append(servers, s)
		}
	}

	if len(servers) > 0 {
		dependencies[ingressKey] = mapFromServerList(servers)
	}

	ingressObjectMap := involvedIngressObjects(servers)
	for ingressKey := range ingressObjectMap {
		if _, ok := dependencies[ingressKey]; !ok {
			dependencyDependencies, err := c.dependencyMap(ingressKey, nil, dependencies)
			if err != nil {
				return nil, err
			}
			for k, v := range dependencyDependencies {
				dependencies[k] = v
			}
		}
	}
	return dependencies, nil
}

func dependencyMapLogEntry(dependencies map[string]map[string]*pb.ServerConfig) (entry *log.Entry) {
	for ingressKey, serverMap := range dependencies {
		servers := []string{}
		for serverName := range serverMap {
			servers = append(servers, serverName)
		}

		if entry == nil {
			entry = log.WithField("ing:"+ingressKey, strings.Join(servers, ", "))
		} else {
			entry = entry.WithField("ing:"+ingressKey, strings.Join(servers, ", "))
		}
	}
	return
}

func (c *configurator) IngressUpdated(updatedIngressKey string) (err error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.mainConfig == nil {
		// no main config
		// do not requeue
		c.log.
			WithField("ingress", updatedIngressKey).
			Info("no main config loaded, skipping")
		return nil
	}

	updated := map[string]map[string]bool{}
	updatedServerNames := []string{}
	mergeList := collision.MergeList{}

	// First Ingress/Updated Ingress
	updatedIngEx, err := c.ingExStore.GetIngressEx(updatedIngressKey)
	if err != nil {
		return
	}
	if updatedIngEx != nil {
		var updatedServers []*config.Server
		updatedServers, err = c.parseServerConfig(updatedIngEx)
		if err != nil {
			return err
		}
		for _, server := range updatedServers {
			if _, ok := updated[updatedIngressKey]; !ok {
				updated[updatedIngressKey] = map[string]bool{}
			}
			updated[updatedIngressKey][server.Name] = true
			updatedServerNames = append(updatedServerNames, server.Name)
		}

		mergeList = append(mergeList, collision.IngressConfig{
			Ingress: updatedIngEx.Ingress,
			Servers: updatedServers,
		})
	}

	dependencyMap, err := c.dependencyMap(updatedIngressKey, updatedServerNames, nil)
	if err != nil {
		return
	}

	if len(dependencyMap) > 1 {
		dependencyMapLogEntry(dependencyMap).WithField("ingress", updatedIngressKey).Info("has dependencies")
		for ingressKey := range dependencyMap {
			if ingressKey == updatedIngressKey {
				continue
			}

			ingEx, gerr := c.ingExStore.GetIngressEx(ingressKey)
			if gerr != nil {
				return gerr
			}

			servers, perr := c.parseServerConfig(ingEx)
			if perr != nil {
				return perr
			}
			for _, server := range servers {
				if _, ok := updated[ingressKey]; !ok {
					updated[ingressKey] = map[string]bool{}
				}
				updated[ingressKey][server.Name] = true
			}

			if len(servers) > 0 {
				mergeList = append(mergeList, collision.IngressConfig{
					Ingress: ingEx.Ingress,
					Servers: servers,
				})
			}
		}
	} else {
		c.log.WithField("ingress", updatedIngressKey).Info("has no dependencies")
	}

	mergedServerConfigs, err := c.ch.Resolve(mergeList)
	if err != nil {
		return err
	}
	for _, updated := range mergedServerConfigs {
		proto, err := c.configurator.RenderServerConfig(&updated)
		if err != nil {
			return err
		}
		err = c.scs.Put(proto)
		if err != nil {
			return err
		}
		c.log.
			WithField("host", updated.Server.Name).
			Debug("updated host")
	}

	// check if configs that where once generated
	// from this ingress can be deleted
	for ingressKey, serverMap := range dependencyMap {
		if _, ingressUpdated := updated[ingressKey]; !ingressUpdated {
			for _, server := range serverMap {
				if len(server.Meta) == 1 {
					err := c.scs.Delete(server)
					if err != nil {
						return err
					}
					c.log.
						WithField("host", server.Name).
						Debug("deleted host")
				}
			}
		} else {
			for serverName, server := range serverMap {
				if _, serverUpdated := updated[ingressKey][serverName]; !serverUpdated && len(server.Meta) == 1 {
					err := c.scs.Delete(server)
					if err != nil {
						return err
					}
					c.log.
						WithField("host", server.Name).
						Debug("deleted host")
				}
			}
		}
	}

	return nil
}

func (c *configurator) handleIngressExValidationError(err *parser.IngressExValidationError) {
	if err.IngressError != nil {
		for _, e := range err.IngressError.Errors {
			c.recorder.Event(err.IngressError.Object, api_v1.EventTypeWarning, "Config Error", e.Error())
		}
	}

	for _, s := range err.SecretErrors {
		for _, e := range err.IngressError.Errors {
			c.recorder.Event(s.Object, api_v1.EventTypeWarning, "Config Error", e.Error())
		}
	}
}

func (c *configurator) getExistingServerConfigsAsMap() (map[string]*pb.ServerConfig, error) {
	existingConfigs, err := c.scs.List()
	if err != nil {
		return nil, err
	}

	existingMap := map[string]*pb.ServerConfig{}
	for _, existingConfig := range existingConfigs {
		existingMap[existingConfig.Name] = existingConfig
	}
	return existingMap, nil
}
