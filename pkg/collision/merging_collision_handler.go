package collision

import (
	"sort"

	log "github.com/sirupsen/logrus"
	"github.com/thetechnick/nginx-ingress/pkg/config"
	"github.com/thetechnick/nginx-ingress/pkg/storage/pb"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

type mergingCollisionHandler struct {
	log *log.Entry
}

// NewMergingCollisionHandler returns a CollisionHandler
// which merges the declaration of multiple ingress objects
func NewMergingCollisionHandler() Handler {
	return &mergingCollisionHandler{
		log: log.WithField("module", "MergingCollisionHandler"),
	}
}

func (m *mergingCollisionHandler) Resolve(mergeList MergeList) (merged []MergedIngressConfig, err error) {
	sort.Sort(mergeList)
	merged = []MergedIngressConfig{}

	hosts := []string{}
	hostServerConfigMap := map[string][]*config.Server{}
	hostIngressMap := map[string][]*v1beta1.Ingress{}
	updatedIngressKeys := map[string]bool{}
	for _, change := range mergeList {
		ingressKey, err := config.KeyFunc(change.Ingress)
		if err != nil {
			return nil, err
		}
		updatedIngressKeys[ingressKey] = true

		for _, server := range change.Servers {
			if _, ok := hostServerConfigMap[server.Name]; !ok {
				hostServerConfigMap[server.Name] = []*config.Server{}
			}
			hostServerConfigMap[server.Name] = append(hostServerConfigMap[server.Name], server)

			if _, ok := hostIngressMap[server.Name]; !ok {
				hosts = append(hosts, server.Name)
				hostIngressMap[server.Name] = []*v1beta1.Ingress{}
			}
			hostIngressMap[server.Name] = append(hostIngressMap[server.Name], change.Ingress)
		}
	}

	m.log.WithField("ings", updatedIngressKeys).WithField("hosts", hosts).Debug("Merging configs")

	for _, host := range hosts {
		serverConfigs := hostServerConfigMap[host]
		var baseServer config.Server
		for i, server := range serverConfigs {
			if i == 0 {
				baseServer = *server
			} else {
				baseServer = *(m.mergeServers(baseServer, server))
			}
		}
		baseServer.Upstreams = m.getUpstreamsForServer(&baseServer)
		mergedConfig := MergedIngressConfig{
			Server:  &baseServer,
			Ingress: hostIngressMap[host],
		}
		merged = append(merged, mergedConfig)
	}

	return
}

func (m *mergingCollisionHandler) getUpstreamsForServer(server *config.Server) []config.Upstream {
	tmp := map[string]config.Upstream{}
	for _, location := range server.Locations {
		tmp[location.Upstream.Name] = location.Upstream
	}

	result := []config.Upstream{}
	for _, upstream := range tmp {
		result = append(result, upstream)
	}
	return result
}

func (m *mergingCollisionHandler) mergeServers(base config.Server, merge *config.Server) *config.Server {
	locationMap := map[string]config.Location{}
	for _, location := range base.Locations {
		locationMap[location.Path] = location
	}
	for _, location := range merge.Locations {
		locationMap[location.Path] = location
	}

	if merge.SSL {
		base.SSL = true
		base.SSLCertificate = merge.SSLCertificate
		base.SSLCertificateKey = merge.SSLCertificateKey
	}

	// files
	if len(merge.Files) > 0 {
		filesMap := map[string]*pb.File{}
		if base.Files == nil {
			base.Files = []*pb.File{}
		} else {
			for _, file := range base.Files {
				filesMap[file.Name] = file
			}
		}
		for _, file := range merge.Files {
			filesMap[file.Name] = file
		}
		files := []*pb.File{}
		for _, file := range filesMap {
			files = append(files, file)
		}
		base.Files = files
	}

	if merge.HTTP2 {
		base.HTTP2 = true
	}
	if merge.HSTS {
		base.HSTS = true
		base.HSTSMaxAge = merge.HSTSMaxAge
		base.HSTSIncludeSubdomains = merge.HSTSIncludeSubdomains
	}

	base.Locations = []config.Location{}
	for _, location := range locationMap {
		base.Locations = append(base.Locations, location)
	}
	return &base
}
