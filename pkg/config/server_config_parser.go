package config

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/thetechnick/nginx-ingress/pkg/storage/pb"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

// EmptyHost is used when no host is set
const EmptyHost = ""

// ServerConfigParser creates new ServerConfigs
type ServerConfigParser interface {
	Parse(
		gCfg GlobalConfig,
		ingCfg IngressConfig,
		tlsCerts map[string]*pb.File,
		endpoints map[string][]string,
	) (servers []*Server, warning, err error)
}

// NewServerConfigParser returns a new serverConfigParser
func NewServerConfigParser() ServerConfigParser {
	return &serverConfigParser{}
}

type serverConfigParser struct{}

func (p *serverConfigParser) Parse(
	gCfg GlobalConfig,
	ingCfg IngressConfig,
	tlsCerts map[string]*pb.File,
	endpoints map[string][]string,
) (servers []*Server, warning, err error) {
	servers = []*Server{}
	warnings := []error{}

	ing := ingCfg.Ingress
	for _, rule := range ing.Spec.Rules {
		if rule.IngressRuleValue.HTTP == nil {
			continue
		}

		serverName := rule.Host
		if rule.Host == EmptyHost {
			log.
				WithField("namespace", ing.Namespace).
				WithField("name", ing.Name).
				Warning("Host field of ingress rule is empty")
		}

		var locations []Location
		upstreams := map[string]Upstream{}
		rootLocation := false

		for _, path := range rule.HTTP.Paths {
			upsName := getNameForUpstream(ing, rule.Host, path.Backend.ServiceName)

			if _, exists := upstreams[upsName]; !exists {
				upstream, err := createUpstream(ing, endpoints, upsName, &path.Backend, ing.Namespace)
				if err != nil {
					warnings = append(warnings, err)
				}
				upstreams[upsName] = upstream
			}

			locPath := pathOrDefault(path.Path)
			if ingCfg.LocationModifier != nil {
				locPath = fmt.Sprintf("%s %s", *ingCfg.LocationModifier, locPath)
			}
			loc := CreateLocation(
				locPath,
				upstreams[upsName],
				&gCfg,
				&ingCfg,
				ingCfg.WebsocketServices[path.Backend.ServiceName],
				ingCfg.Rewrites[path.Backend.ServiceName],
				ingCfg.SSLServices[path.Backend.ServiceName],
			)
			locations = append(locations, loc)

			if loc.Path == "/" {
				rootLocation = true
			}
		}

		if rootLocation == false && ing.Spec.Backend != nil {
			upsName := getNameForUpstream(ing, EmptyHost, ing.Spec.Backend.ServiceName)
			loc := CreateLocation(
				pathOrDefault("/"),
				upstreams[upsName],
				&gCfg,
				&ingCfg,
				ingCfg.WebsocketServices[ing.Spec.Backend.ServiceName],
				ingCfg.Rewrites[ing.Spec.Backend.ServiceName],
				ingCfg.SSLServices[ing.Spec.Backend.ServiceName],
			)
			locations = append(locations, loc)
		}

		server := CreateServerConfig(&gCfg, &ingCfg)
		server.Name = serverName
		server.Locations = locations
		server.Upstreams = upstreamMapToList(upstreams)
		if pemFile, ok := tlsCerts[serverName]; ok {
			server.SSL = true
			server.SSLCertificate = pemFile.Name
			server.SSLCertificateKey = pemFile.Name
			server.Files = append(server.Files, pemFile)
		}
		servers = append(servers, server)
	}

	if len(ing.Spec.Rules) == 0 && ing.Spec.Backend != nil {
		upsName := getNameForUpstream(ing, EmptyHost, ing.Spec.Backend.ServiceName)
		upstream, err := createUpstream(ing, endpoints, upsName, ing.Spec.Backend, ing.Namespace)
		if err != nil {
			warnings = append(warnings, err)
		}
		location := CreateLocation(
			pathOrDefault("/"),
			upstream,
			&gCfg,
			&ingCfg,
			ingCfg.WebsocketServices[ing.Spec.Backend.ServiceName],
			ingCfg.Rewrites[ing.Spec.Backend.ServiceName],
			ingCfg.SSLServices[ing.Spec.Backend.ServiceName],
		)

		server := CreateServerConfig(&gCfg, &ingCfg)
		server.Name = EmptyHost
		server.Locations = []Location{location}
		server.Upstreams = []Upstream{upstream}
		if pemFile, ok := tlsCerts[EmptyHost]; ok {
			server.SSL = true
			server.SSLCertificate = pemFile.Name
			server.SSLCertificateKey = pemFile.Name
			server.Files = append(server.Files, pemFile)
		}
	}

	if len(warnings) > 0 {
		warning = ValidationError(warnings)
	}

	return
}

func upstreamMapToList(upstreams map[string]Upstream) (result []Upstream) {
	for _, up := range upstreams {
		result = append(result, up)
	}
	return
}

func pathOrDefault(path string) string {
	if path == "" {
		return "/"
	}
	return path
}

func getNameForUpstream(ing *v1beta1.Ingress, host string, service string) string {
	return fmt.Sprintf("%v-%v-%v-%v", ing.Namespace, ing.Name, host, service)
}

func createUpstream(
	ing *v1beta1.Ingress,
	endpoints map[string][]string,
	name string,
	backend *v1beta1.IngressBackend,
	namespace string,
) (Upstream, error) {
	ups := NewUpstreamWithDefaultServer(name)

	if endps, exists := endpoints[backend.ServiceName+backend.ServicePort.String()]; exists {
		var upsServers []UpstreamServer
		for _, endp := range endps {
			addressport := strings.Split(endp, ":")
			upsServers = append(upsServers, UpstreamServer{
				Address: addressport[0],
				Port:    addressport[1],
			})
		}

		if len(upsServers) > 0 {
			// override default upstream
			ups.UpstreamServers = upsServers
			return ups, nil
		}
	}

	return ups, fmt.Errorf(
		"no active endpoints for service:port %s/%s:%s",
		ing.Namespace,
		backend.ServiceName,
		backend.ServicePort.String(),
	)
}
