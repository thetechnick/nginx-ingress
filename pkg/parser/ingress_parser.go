package parser

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/thetechnick/nginx-ingress/pkg/config"
	"github.com/thetechnick/nginx-ingress/pkg/errors"
	"github.com/thetechnick/nginx-ingress/pkg/storage/pb"
	"github.com/thetechnick/nginx-ingress/pkg/util"
	extensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

const emptyHost = ""

// IngressAnnotationError is a config error for annotation of the Ingress object
type IngressAnnotationError struct {
	Annotation      string
	ValidationError error
}

func (e *IngressAnnotationError) Error() string {
	return fmt.Sprintf("Skipping annotation %q: %v", e.Annotation, e.ValidationError)
}

// IngressParser parses ingress objects
type IngressParser interface {
	Parse(ingCfg config.Config, ingEx *config.IngressEx, pems map[string]*pb.TLSCertificate) ([]*config.Server, error)
}

// NewIngressParser returns a new IngressParser instance
func NewIngressParser() IngressParser {
	return &ingressParser{}
}

type ingressParser struct{}

func (p *ingressParser) Parse(ingCfg config.Config, ingEx *config.IngressEx, pems map[string]*pb.TLSCertificate) ([]*config.Server, error) {
	ing := ingEx.Ingress
	errs := []error{}
	if serverTokens, exists, err := util.GetMapKeyAsBool(ing.Annotations, "nginx.org/server-tokens"); exists {
		if err != nil {
			errs = append(errs, &IngressAnnotationError{"nginx.org/server-tokens", err})
		} else {
			ingCfg.ServerTokens = serverTokens
		}
	}

	if serverSnippets, exists := util.GetMapKeyAsStringSlice(ing.Annotations, "nginx.org/server-snippets", ing, "\n"); exists {
		ingCfg.ServerSnippets = serverSnippets
	}
	if locationSnippets, exists := util.GetMapKeyAsStringSlice(ing.Annotations, "nginx.org/location-snippets", ing, "\n"); exists {
		ingCfg.LocationSnippets = locationSnippets
	}

	if proxyConnectTimeout, exists := ing.Annotations["nginx.org/proxy-connect-timeout"]; exists {
		ingCfg.ProxyConnectTimeout = proxyConnectTimeout
	}
	if proxyReadTimeout, exists := ing.Annotations["nginx.org/proxy-read-timeout"]; exists {
		ingCfg.ProxyReadTimeout = proxyReadTimeout
	}
	if proxyHideHeaders, exists := util.GetMapKeyAsStringSlice(ing.Annotations, "nginx.org/proxy-hide-headers", ing, ","); exists {
		ingCfg.ProxyHideHeaders = proxyHideHeaders
	}
	if proxyPassHeaders, exists := util.GetMapKeyAsStringSlice(ing.Annotations, "nginx.org/proxy-pass-headers", ing, ","); exists {
		ingCfg.ProxyPassHeaders = proxyPassHeaders
	}
	if clientMaxBodySize, exists := ing.Annotations["nginx.org/client-max-body-size"]; exists {
		ingCfg.ClientMaxBodySize = clientMaxBodySize
	}
	if HTTP2, exists, err := util.GetMapKeyAsBool(ing.Annotations, "nginx.org/http2"); exists {
		if err != nil {
			errs = append(errs, &IngressAnnotationError{"nginx.org/http2", err})
		} else {
			ingCfg.HTTP2 = HTTP2
		}
	}
	if redirectToHTTPS, exists, err := util.GetMapKeyAsBool(ing.Annotations, "nginx.org/redirect-to-https"); exists {
		if err != nil {
			errs = append(errs, &IngressAnnotationError{"nginx.org/redirect-to-https", err})
		} else {
			ingCfg.RedirectToHTTPS = redirectToHTTPS
		}
	}
	if proxyBuffering, exists, err := util.GetMapKeyAsBool(ing.Annotations, "nginx.org/proxy-buffering"); exists {
		if err != nil {
			errs = append(errs, &IngressAnnotationError{"nginx.org/proxy-buffering", err})
		} else {
			ingCfg.ProxyBuffering = proxyBuffering
		}
	}

	if hsts, exists, err := util.GetMapKeyAsBool(ing.Annotations, "nginx.org/hsts"); exists {
		if err != nil {
			errs = append(errs, &IngressAnnotationError{"nginx.org/hsts", err})
		} else {
			parsingErrors := false

			hstsMaxAge, existsMA, err := util.GetMapKeyAsInt(ing.Annotations, "nginx.org/hsts-max-age")
			if existsMA && err != nil {
				errs = append(errs, &IngressAnnotationError{"nginx.org/hsts-max-age", err})
				parsingErrors = true
			}
			hstsIncludeSubdomains, existsIS, err := util.GetMapKeyAsBool(ing.Annotations, "nginx.org/hsts-include-subdomains")
			if existsIS && err != nil {
				errs = append(errs, &IngressAnnotationError{"nginx.org/hsts-include-subdomains", err})
				parsingErrors = true
			}

			if parsingErrors {
				errs = append(errs, fmt.Errorf("Error parsing hsts ingress annotations, skipping all hsts annotations"))
			} else {
				ingCfg.HSTS = hsts
				if existsMA {
					ingCfg.HSTSMaxAge = hstsMaxAge
				}
				if existsIS {
					ingCfg.HSTSIncludeSubdomains = hstsIncludeSubdomains
				}
			}
		}
	}

	if proxyBuffers, exists := ing.Annotations["nginx.org/proxy-buffers"]; exists {
		ingCfg.ProxyBuffers = proxyBuffers
	}
	if proxyBufferSize, exists := ing.Annotations["nginx.org/proxy-buffer-size"]; exists {
		ingCfg.ProxyBufferSize = proxyBufferSize
	}
	if proxyMaxTempFileSize, exists := ing.Annotations["nginx.org/proxy-max-temp-file-size"]; exists {
		ingCfg.ProxyMaxTempFileSize = proxyMaxTempFileSize
	}

	wsServices := getWebsocketServices(ing)
	sslServices := getSSLServices(ing)
	rewrites, err := getRewrites(ing)
	if err != nil {
		errs = append(errs, &IngressAnnotationError{"nginx.org/rewrites", err})
	}

	//
	// build server config
	//
	servers := []*config.Server{}

	for _, rule := range ing.Spec.Rules {
		if rule.IngressRuleValue.HTTP == nil {
			continue
		}

		serverName := rule.Host

		if rule.Host == emptyHost {
			log.
				WithField("namespace", ing.Namespace).
				WithField("name", ing.Name).
				Warning("Host field of ingress rule is empty")
		}

		var locations []config.Location
		upstreams := map[string]config.Upstream{}
		rootLocation := false

		for _, path := range rule.HTTP.Paths {
			upsName := getNameForUpstream(ing, rule.Host, path.Backend.ServiceName)

			if _, exists := upstreams[upsName]; !exists {
				upstream := createUpstream(ingEx, upsName, &path.Backend, ing.Namespace)
				upstreams[upsName] = upstream
			}

			loc := config.CreateLocation(pathOrDefault(path.Path), upstreams[upsName], &ingCfg, wsServices[path.Backend.ServiceName], rewrites[path.Backend.ServiceName], sslServices[path.Backend.ServiceName])
			locations = append(locations, loc)

			if loc.Path == "/" {
				rootLocation = true
			}
		}

		if rootLocation == false && ing.Spec.Backend != nil {
			upsName := getNameForUpstream(ing, emptyHost, ing.Spec.Backend.ServiceName)
			loc := config.CreateLocation(pathOrDefault("/"), upstreams[upsName], &ingCfg, wsServices[ing.Spec.Backend.ServiceName], rewrites[ing.Spec.Backend.ServiceName], sslServices[ing.Spec.Backend.ServiceName])
			locations = append(locations, loc)
		}

		server := config.Server{
			Name:                  serverName,
			Locations:             locations,
			Upstreams:             upstreamMapToList(upstreams),
			ServerTokens:          ingCfg.ServerTokens,
			HTTP2:                 ingCfg.HTTP2,
			RedirectToHTTPS:       ingCfg.RedirectToHTTPS,
			ProxyProtocol:         ingCfg.ProxyProtocol,
			HSTS:                  ingCfg.HSTS,
			HSTSMaxAge:            ingCfg.HSTSMaxAge,
			HSTSIncludeSubdomains: ingCfg.HSTSIncludeSubdomains,
			RealIPHeader:          ingCfg.RealIPHeader,
			SetRealIPFrom:         ingCfg.SetRealIPFrom,
			RealIPRecursive:       ingCfg.RealIPRecursive,
			ProxyHideHeaders:      ingCfg.ProxyHideHeaders,
			ProxyPassHeaders:      ingCfg.ProxyPassHeaders,
			ServerSnippets:        ingCfg.ServerSnippets,
		}

		if pemFile, ok := pems[serverName]; ok {
			server.SSL = true
			server.SSLCertificate = pemFile.Name
			server.SSLCertificateKey = pemFile.Name
			server.TLSCertificateFile = pemFile
		}

		servers = append(servers, &server)
	}

	if len(ing.Spec.Rules) == 0 && ing.Spec.Backend != nil {
		upsName := getNameForUpstream(ing, emptyHost, ing.Spec.Backend.ServiceName)
		upstream := createUpstream(ingEx, upsName, ing.Spec.Backend, ing.Namespace)
		location := config.CreateLocation(pathOrDefault("/"), upstream, &ingCfg, wsServices[ing.Spec.Backend.ServiceName], rewrites[ing.Spec.Backend.ServiceName], sslServices[ing.Spec.Backend.ServiceName])

		server := config.Server{
			Name:                  emptyHost,
			ServerTokens:          ingCfg.ServerTokens,
			Upstreams:             []config.Upstream{upstream},
			Locations:             []config.Location{location},
			HTTP2:                 ingCfg.HTTP2,
			RedirectToHTTPS:       ingCfg.RedirectToHTTPS,
			ProxyProtocol:         ingCfg.ProxyProtocol,
			HSTS:                  ingCfg.HSTS,
			HSTSMaxAge:            ingCfg.HSTSMaxAge,
			HSTSIncludeSubdomains: ingCfg.HSTSIncludeSubdomains,
			RealIPHeader:          ingCfg.RealIPHeader,
			SetRealIPFrom:         ingCfg.SetRealIPFrom,
			RealIPRecursive:       ingCfg.RealIPRecursive,
			ProxyHideHeaders:      ingCfg.ProxyHideHeaders,
			ProxyPassHeaders:      ingCfg.ProxyPassHeaders,
			ServerSnippets:        ingCfg.ServerSnippets,
		}

		if pemFile, ok := pems[emptyHost]; ok {
			server.SSL = true
			server.SSLCertificate = pemFile.Name
			server.SSLCertificateKey = pemFile.Name
			server.TLSCertificateFile = pemFile
		}

		servers = append(servers, &server)
	}

	if len(errs) > 0 {
		return servers, errors.WrapInObjectContext(ValidationError(errs), ing)
	}
	return servers, nil
}

func pathOrDefault(path string) string {
	if path == "" {
		return "/"
	}
	return path
}

func getNameForUpstream(ing *extensions.Ingress, host string, service string) string {
	return fmt.Sprintf("%v-%v-%v-%v", ing.Namespace, ing.Name, host, service)
}

func getWebsocketServices(ing *extensions.Ingress) map[string]bool {
	wsServices := make(map[string]bool)

	if services, exists := ing.Annotations["nginx.org/websocket-services"]; exists {
		for _, svc := range strings.Split(services, ",") {
			wsServices[svc] = true
		}
	}

	return wsServices
}

func getRewrites(ing *extensions.Ingress) (rewrites map[string]string, err error) {
	rewrites = make(map[string]string)

	if services, exists := ing.Annotations["nginx.org/rewrites"]; exists {
		for _, svc := range strings.Split(services, ";") {
			serviceName, rewrite, err := parseRewrites(svc)
			if err != nil {
				return rewrites, err
			}
			rewrites[serviceName] = rewrite
		}
	}

	return
}

func parseRewrites(service string) (serviceName string, rewrite string, err error) {
	parts := strings.SplitN(service, " ", 2)

	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid rewrite format: %s", service)
	}

	svcNameParts := strings.Split(parts[0], "=")
	if len(svcNameParts) != 2 {
		return "", "", fmt.Errorf("invalid rewrite format: %s", svcNameParts)
	}

	rwPathParts := strings.Split(parts[1], "=")
	if len(rwPathParts) != 2 {
		return "", "", fmt.Errorf("invalid rewrite format: %s", rwPathParts)
	}

	return svcNameParts[1], rwPathParts[1], nil
}

func getSSLServices(ing *extensions.Ingress) map[string]bool {
	sslServices := make(map[string]bool)

	if services, exists := ing.Annotations["nginx.org/ssl-services"]; exists {
		for _, svc := range strings.Split(services, ",") {
			sslServices[svc] = true
		}
	}

	return sslServices
}

func createUpstream(ingEx *config.IngressEx, name string, backend *extensions.IngressBackend, namespace string) config.Upstream {
	ups := config.NewUpstreamWithDefaultServer(name)

	endps, exists := ingEx.Endpoints[backend.ServiceName+backend.ServicePort.String()]
	if exists {
		var upsServers []config.UpstreamServer
		for _, endp := range endps {
			addressport := strings.Split(endp, ":")
			upsServers = append(upsServers, config.UpstreamServer{
				Address: addressport[0],
				Port:    addressport[1],
			})
		}
		if len(upsServers) > 0 {
			ups.UpstreamServers = upsServers
		}
	}

	return ups
}

func upstreamMapToList(upstreams map[string]config.Upstream) (result []config.Upstream) {
	for _, up := range upstreams {
		result = append(result, up)
	}
	return
}
