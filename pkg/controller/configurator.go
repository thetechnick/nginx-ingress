package controller

import (
	"fmt"
	"path"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/thetechnick/nginx-ingress/pkg/collision"
	"github.com/thetechnick/nginx-ingress/pkg/config"
	"github.com/thetechnick/nginx-ingress/pkg/errors"
	"github.com/thetechnick/nginx-ingress/pkg/renderer"
	"github.com/thetechnick/nginx-ingress/pkg/storage"
	"github.com/thetechnick/nginx-ingress/pkg/storage/pb"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	api_v1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
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
	ingressAccessor IngressAccessor,
	secretAccessor SecretAccessor,
	endpointsAccessor EndpointsAccessor,

	recorder record.EventRecorder,
	mcs storage.MainConfigStorage,
	scs storage.ServerConfigStorage,
) Configurator {
	return &configurator{
		scs: scs,
		mcs: mcs,

		ingressAccessor:   ingressAccessor,
		secretAccessor:    secretAccessor,
		endpointsAccessor: endpointsAccessor,

		ingParser:          config.NewIngressConfigParser(),
		tlsSecretParser:    config.NewSecretParser(),
		configMapParser:    config.NewConfigMapParser(),
		serverConfigParser: config.NewServerConfigParser(),

		ch:           collision.NewMergingCollisionHandler(),
		configurator: renderer.NewRenderer(),
		recorder:     recorder,
		log:          log.WithField("module", "Configurator"),
	}
}

type configurator struct {
	log        *log.Entry
	mainConfig *config.GlobalConfig
	mutex      sync.Mutex

	mcs storage.MainConfigStorage
	scs storage.ServerConfigStorage

	// k8s accessors
	ingressAccessor   IngressAccessor
	secretAccessor    SecretAccessor
	endpointsAccessor EndpointsAccessor

	tlsSecretParser    config.SecretParser
	ingParser          config.IngressConfigParser
	configMapParser    config.ConfigMapParser
	serverConfigParser config.ServerConfigParser

	ch           collision.Handler
	configurator renderer.Renderer
	recorder     record.EventRecorder
}

func (c *configurator) ConfigUpdated(cfgm *api_v1.ConfigMap) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	nginxConfig, err := c.configMapParser.Parse(cfgm)
	if err != nil {
		c.recordError("Config Error", err)
		if nginxConfig == nil {
			return err
		}
	}
	c.mainConfig = nginxConfig

	configUpdate, err := c.configurator.RenderMainConfig(
		renderer.MainConfigTemplateDataFromIngressConfig(nginxConfig),
	)
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

func (c *configurator) recordError(reason string, err error) {
	if err == nil {
		return
	}

	cerr, ok := err.(errors.ErrObjectContext)
	if !ok {
		c.log.
			WithError(err).
			Error("Could not record error event due to missing context")
		return
	}

	if werr, ok := cerr.WrappedError().(config.ValidationError); !ok {
		for _, e := range werr {
			c.recorder.Event(cerr.Object(), api_v1.EventTypeWarning, reason, e.Error())
		}
	}

	c.recorder.Event(
		cerr.Object(),
		api_v1.EventTypeWarning,
		reason,
		cerr.Error(),
	)
}

func (c *configurator) serverConfigForIngressKey(ingressKey string) (
	ingress *v1beta1.Ingress,
	servers []*config.Server,
	err error,
) {
	ingress, err = c.ingressAccessor.GetByKey(ingressKey)
	if err != nil {
		if api_errors.IsNotFound(err) {
			return nil, nil, nil
		}
		return
	}
	ingressCfg, warning, cfgErr := c.ingParser.Parse(ingress)
	if warning != nil {
		c.recordError("Config Warnings", warning)
	}
	if cfgErr != nil {
		err = cfgErr
		return
	}

	// get secrets
	tlsSecrets := map[string]*pb.File{}
	for _, tls := range ingress.Spec.TLS {
		var secret *api_v1.Secret
		if secret, err = c.secretAccessor.Get(ingress.Namespace, tls.SecretName); err != nil {
			return
		}

		var tlsCert []byte
		if tlsCert, err = c.tlsSecretParser.Parse(secret); err != nil {
			return
		}

		for _, host := range tls.Hosts {
			tlsName := path.Join(storage.CertificatesDir, fmt.Sprintf("%s.pem", host))
			tlsSecrets[host] = &pb.File{
				Name:    tlsName,
				Content: tlsCert,
			}
		}
		if len(tls.Hosts) == 0 {
			tlsName := path.Join(storage.CertificatesDir, "default.pem")
			tlsSecrets[config.EmptyHost] = &pb.File{
				Name:    tlsName,
				Content: tlsCert,
			}
		}
	}

	// get endpoints
	endpoints := map[string][]string{}
	if ingress.Spec.Backend != nil {
		endps, gerr := c.endpointsAccessor.GetEndpointsForIngressBackend(ingress.Spec.Backend, ingress.Namespace)
		if gerr != nil {
			c.log.
				WithField("namespace", ingress.Namespace).
				WithField("name", ingress.Name).
				WithError(gerr).
				Errorf("Error retrieving endpoints for ingress backend: %v", ingress.Spec.Backend.ServiceName)
		} else {
			endpoints[ingress.Spec.Backend.ServiceName+ingress.Spec.Backend.ServicePort.String()] = endps
		}
	}
	for _, rule := range ingress.Spec.Rules {
		if rule.IngressRuleValue.HTTP == nil {
			continue
		}

		for _, path := range rule.HTTP.Paths {
			endps, gerr := c.endpointsAccessor.GetEndpointsForIngressBackend(&path.Backend, ingress.Namespace)
			if gerr != nil {
				log.
					WithField("namespace", ingress.Namespace).
					WithField("name", ingress.Name).
					WithError(gerr).
					Errorf("Error retrieving endpoints for ingress backend: %v", path.Backend.ServiceName)
			} else {
				endpoints[path.Backend.ServiceName+path.Backend.ServicePort.String()] = endps
			}
		}
	}

	mainCfg := *c.mainConfig
	var scWarning error
	servers, scWarning, err = c.serverConfigParser.Parse(mainCfg, *ingressCfg, tlsSecrets, endpoints)
	if scWarning != nil {
		c.recordError("Config Warning", scWarning)
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
	updatedIngress, updatedServers, err := c.serverConfigForIngressKey(updatedIngressKey)
	if err != nil {
		c.recordError("Config Error", err)
		return
	}

	if updatedIngress != nil {
		c.log.
			WithField("ingress", updatedIngressKey).
			Info("updating")

		for _, server := range updatedServers {
			if _, ok := updated[updatedIngressKey]; !ok {
				updated[updatedIngressKey] = map[string]bool{}
			}
			updated[updatedIngressKey][server.Name] = true
			updatedServerNames = append(updatedServerNames, server.Name)
		}

		mergeList = append(mergeList, collision.IngressConfig{
			Ingress: updatedIngress,
			Servers: updatedServers,
		})
	} else {
		c.log.
			WithField("ingress", updatedIngressKey).
			Info("deleting")
	}

	dependencyMap, err := c.dependencyMap(updatedIngressKey, updatedServerNames, nil)
	if err != nil {
		c.recordError("Error mapping dependencies", err)
		return
	}

	if len(dependencyMap) > 1 {
		dependencyMapLogEntry(dependencyMap).WithField("ingress", updatedIngressKey).Info("has dependencies")
		for ingressKey := range dependencyMap {
			if ingressKey == updatedIngressKey {
				continue
			}

			ing, servers, gerr := c.serverConfigForIngressKey(ingressKey)
			if gerr != nil {
				return gerr
			}
			if ing == nil {
				continue
			}

			for _, server := range servers {
				if _, ok := updated[ingressKey]; !ok {
					updated[ingressKey] = map[string]bool{}
				}
				updated[ingressKey][server.Name] = true
			}

			if len(servers) > 0 {
				mergeList = append(mergeList, collision.IngressConfig{
					Ingress: ing,
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
