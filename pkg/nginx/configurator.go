package nginx

import (
	"fmt"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/config"
	api_v1 "k8s.io/client-go/pkg/api/v1"
	extensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

const emptyHost = ""

// Configurator transforms an Ingress resource into NGINX Configuration
type Configurator struct {
	nginx            *Controller
	config           *config.Config
	collisionHandler CollisionHandler
	lock             sync.Mutex
}

// NewConfigurator creates a new Configurator
func NewConfigurator(nginx *Controller, collisionHandler CollisionHandler, config *config.Config) *Configurator {
	cnf := Configurator{
		nginx:            nginx,
		config:           config,
		collisionHandler: collisionHandler,
	}

	return &cnf
}

// AddOrUpdateDHParam adds/updates the dhparam file
func (cnf *Configurator) AddOrUpdateDHParam(content string) (string, error) {
	return cnf.nginx.AddOrUpdateDHParam(content)
}

// AddOrUpdateIngress adds or updates NGINX configuration for an Ingress resource
func (cnf *Configurator) AddOrUpdateIngress(name string, ingEx *IngressEx) {
	cnf.lock.Lock()
	defer cnf.lock.Unlock()

	pems := cnf.updateCertificates(ingEx)
	generatedServers := cnf.generateNginxCfg(ingEx, pems)
	servers, err := cnf.collisionHandler.AddConfigs(ingEx.Ingress, generatedServers)
	if err != nil {
		log.
			WithField("namespace", ingEx.Ingress.Namespace).
			WithField("name", ingEx.Ingress.Name).
			WithError(err).
			Error("Error when checking generated servers for collisions")
		return
	}
	for _, server := range servers {
		cnf.nginx.AddOrUpdateConfig(server.Name, server)
	}
	if err := cnf.nginx.Reload(); err != nil {
		log.
			WithField("namespace", ingEx.Ingress.Namespace).
			WithField("name", ingEx.Ingress.Name).
			WithError(err).
			Error("Error when adding or updating ingress")
	}
}

func (cnf *Configurator) updateCertificates(ingEx *IngressEx) map[string]string {
	pems := make(map[string]string)

	for _, tls := range ingEx.Ingress.Spec.TLS {
		secretName := tls.SecretName
		secret, exist := ingEx.Secrets[secretName]
		if !exist {
			continue
		}
		cert, ok := secret.Data[api_v1.TLSCertKey]
		if !ok {
			log.
				WithField("namespace", secret.Namespace).
				WithField("name", secret.Name).
				Error("Secret has no cert, skipping")
			continue
		}
		key, ok := secret.Data[api_v1.TLSPrivateKeyKey]
		if !ok {
			log.
				WithField("namespace", secret.Namespace).
				WithField("name", secret.Name).
				Error("Secret has no private key, skipping")
			continue
		}

		name := ingEx.Ingress.Namespace + "-" + secretName
		pemFileName := cnf.nginx.AddOrUpdateCertAndKey(name, string(cert), string(key))

		for _, host := range tls.Hosts {
			pems[host] = pemFileName
		}
		if len(tls.Hosts) == 0 {
			pems[emptyHost] = pemFileName
		}
	}

	return pems
}
func (cnf *Configurator) generateNginxCfg(ingEx *IngressEx, pems map[string]string) []config.Server {
	ingCfg := cnf.createConfig(ingEx)
	ing := ingEx.Ingress

	wsServices := getWebsocketServices(ingEx)
	rewrites, err := getRewrites(ingEx)
	if err != nil {
		log.
			WithField("namespace", ing.Namespace).
			WithField("name", ing.Name).
			WithField("annotation", "nginx.org/rewrites").
			WithError(err).
			Error("Error parsing ingress annotation, skipping")
	}
	sslServices := getSSLServices(ingEx)
	servers := []config.Server{}

	for _, rule := range ingEx.Ingress.Spec.Rules {
		if rule.IngressRuleValue.HTTP == nil {
			continue
		}

		serverName := rule.Host

		if rule.Host == emptyHost {
			log.
				WithField("namespace", ingEx.Ingress.Namespace).
				WithField("name", ingEx.Ingress.Name).
				Warning("Host field of ingress rule is empty")
		}

		var locations []config.Location
		upstreams := map[string]config.Upstream{}
		rootLocation := false

		for _, path := range rule.HTTP.Paths {
			upsName := getNameForUpstream(ingEx.Ingress, rule.Host, path.Backend.ServiceName)

			if _, exists := upstreams[upsName]; !exists {
				upstream := cnf.createUpstream(ingEx, upsName, &path.Backend, ingEx.Ingress.Namespace)
				upstreams[upsName] = upstream
			}

			loc := createLocation(pathOrDefault(path.Path), upstreams[upsName], &ingCfg, wsServices[path.Backend.ServiceName], rewrites[path.Backend.ServiceName], sslServices[path.Backend.ServiceName])
			locations = append(locations, loc)

			if loc.Path == "/" {
				rootLocation = true
			}
		}

		if rootLocation == false && ingEx.Ingress.Spec.Backend != nil {
			upsName := getNameForUpstream(ingEx.Ingress, emptyHost, ingEx.Ingress.Spec.Backend.ServiceName)
			loc := createLocation(pathOrDefault("/"), upstreams[upsName], &ingCfg, wsServices[ingEx.Ingress.Spec.Backend.ServiceName], rewrites[ingEx.Ingress.Spec.Backend.ServiceName], sslServices[ingEx.Ingress.Spec.Backend.ServiceName])
			locations = append(locations, loc)
		}

		server := config.Server{
			Name:                  serverName,
			Locations:             locations,
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
			server.SSLCertificate = pemFile
			server.SSLCertificateKey = pemFile
		}

		servers = append(servers, server)
	}

	if len(ingEx.Ingress.Spec.Rules) == 0 && ingEx.Ingress.Spec.Backend != nil {
		upsName := getNameForUpstream(ingEx.Ingress, emptyHost, ingEx.Ingress.Spec.Backend.ServiceName)
		upstream := cnf.createUpstream(ingEx, upsName, ingEx.Ingress.Spec.Backend, ingEx.Ingress.Namespace)
		location := createLocation(pathOrDefault("/"), upstream, &ingCfg, wsServices[ingEx.Ingress.Spec.Backend.ServiceName], rewrites[ingEx.Ingress.Spec.Backend.ServiceName], sslServices[ingEx.Ingress.Spec.Backend.ServiceName])

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
			server.SSLCertificate = pemFile
			server.SSLCertificateKey = pemFile
		}

		servers = append(servers, server)
	}

	return servers
}

func (cnf *Configurator) createConfig(ingEx *IngressEx) config.Config {
	ingCfg := *cnf.config
	ing := ingEx.Ingress
	if serverTokens, exists, err := GetMapKeyAsBool(ingEx.Ingress.Annotations, "nginx.org/server-tokens", ingEx.Ingress); exists {
		if err != nil {
			log.
				WithField("namespace", ing.Namespace).
				WithField("name", ing.Name).
				WithField("annotation", "nginx.org/server-tokens").
				WithError(err).
				Error("Error parsing ingress annotation, skipping")
		} else {
			ingCfg.ServerTokens = serverTokens
		}
	}

	if serverSnippets, exists, err := GetMapKeyAsStringSlice(ingEx.Ingress.Annotations, "nginx.org/server-snippets", ingEx.Ingress, "\n"); exists {
		if err != nil {
			log.
				WithField("namespace", ing.Namespace).
				WithField("name", ing.Name).
				WithField("annotation", "nginx.org/server-snippets").
				WithError(err).
				Error("Error parsing ingress annotation, skipping")
		} else {
			ingCfg.ServerSnippets = serverSnippets
		}
	}
	if locationSnippets, exists, err := GetMapKeyAsStringSlice(ingEx.Ingress.Annotations, "nginx.org/location-snippets", ingEx.Ingress, "\n"); exists {
		if err != nil {
			log.
				WithField("namespace", ing.Namespace).
				WithField("name", ing.Name).
				WithField("annotation", "nginx.org/location-snippets").
				WithError(err).
				Error("Error parsing ingress annotation, skipping")
		} else {
			ingCfg.LocationSnippets = locationSnippets
		}
	}

	if proxyConnectTimeout, exists := ingEx.Ingress.Annotations["nginx.org/proxy-connect-timeout"]; exists {
		ingCfg.ProxyConnectTimeout = proxyConnectTimeout
	}
	if proxyReadTimeout, exists := ingEx.Ingress.Annotations["nginx.org/proxy-read-timeout"]; exists {
		ingCfg.ProxyReadTimeout = proxyReadTimeout
	}
	if proxyHideHeaders, exists, err := GetMapKeyAsStringSlice(ingEx.Ingress.Annotations, "nginx.org/proxy-hide-headers", ingEx.Ingress, ","); exists {
		if err != nil {
			log.
				WithField("namespace", ing.Namespace).
				WithField("name", ing.Name).
				WithField("annotation", "nginx.org/proxy-hide-headers").
				WithError(err).
				Error("Error parsing ingress annotation, skipping")
		} else {
			ingCfg.ProxyHideHeaders = proxyHideHeaders
		}
	}
	if proxyPassHeaders, exists, err := GetMapKeyAsStringSlice(ingEx.Ingress.Annotations, "nginx.org/proxy-pass-headers", ingEx.Ingress, ","); exists {
		if err != nil {
			log.
				WithField("namespace", ing.Namespace).
				WithField("name", ing.Name).
				WithField("annotation", "nginx.org/proxy-pass-headers").
				WithError(err).
				Error("Error parsing ingress annotation, skipping")
		} else {
			ingCfg.ProxyPassHeaders = proxyPassHeaders
		}
	}
	if clientMaxBodySize, exists := ingEx.Ingress.Annotations["nginx.org/client-max-body-size"]; exists {
		ingCfg.ClientMaxBodySize = clientMaxBodySize
	}
	if HTTP2, exists, err := GetMapKeyAsBool(ingEx.Ingress.Annotations, "nginx.org/http2", ingEx.Ingress); exists {
		if err != nil {
			log.
				WithField("namespace", ing.Namespace).
				WithField("name", ing.Name).
				WithField("annotation", "nginx.org/http2").
				WithError(err).
				Error("Error parsing ingress annotation, skipping")
		} else {
			ingCfg.HTTP2 = HTTP2
		}
	}
	if redirectToHTTPS, exists, err := GetMapKeyAsBool(ingEx.Ingress.Annotations, "nginx.org/redirect-to-https", ingEx.Ingress); exists {
		if err != nil {
			log.
				WithField("namespace", ing.Namespace).
				WithField("name", ing.Name).
				WithField("annotation", "nginx.org/redirect-to-https").
				WithError(err).
				Error("Error parsing ingress annotation, skipping")
		} else {
			ingCfg.RedirectToHTTPS = redirectToHTTPS
		}
	}
	if proxyBuffering, exists, err := GetMapKeyAsBool(ingEx.Ingress.Annotations, "nginx.org/proxy-buffering", ingEx.Ingress); exists {
		if err != nil {
			log.
				WithField("namespace", ing.Namespace).
				WithField("name", ing.Name).
				WithField("annotation", "nginx.org/proxy-buffering").
				WithError(err).
				Error("Error parsing ingress annotation, skipping")
		} else {
			ingCfg.ProxyBuffering = proxyBuffering
		}
	}

	if hsts, exists, err := GetMapKeyAsBool(ingEx.Ingress.Annotations, "nginx.org/hsts", ingEx.Ingress); exists {
		if err != nil {
			log.
				WithField("namespace", ing.Namespace).
				WithField("name", ing.Name).
				WithField("annotation", "nginx.org/hsts").
				WithError(err).
				Error("Error parsing ingress annotation, skipping")
		} else {
			parsingErrors := false

			hstsMaxAge, existsMA, err := GetMapKeyAsInt(ingEx.Ingress.Annotations, "nginx.org/hsts-max-age", ingEx.Ingress)
			if existsMA && err != nil {
				log.
					WithField("namespace", ing.Namespace).
					WithField("name", ing.Name).
					WithField("annotation", "nginx.org/hsts-max-age").
					WithError(err).
					Error("Error parsing ingress annotation, skipping")
				parsingErrors = true
			}
			hstsIncludeSubdomains, existsIS, err := GetMapKeyAsBool(ingEx.Ingress.Annotations, "nginx.org/hsts-include-subdomains", ingEx.Ingress)
			if existsIS && err != nil {
				log.
					WithField("namespace", ing.Namespace).
					WithField("name", ing.Name).
					WithField("annotation", "nginx.org/hsts-include-subdomains").
					WithError(err).
					Error("Error parsing ingress annotation, skipping")
				parsingErrors = true
			}

			if parsingErrors {
				log.
					WithField("namespace", ing.Namespace).
					WithField("name", ing.Name).
					WithField("annotation", "hsts").
					WithError(err).
					Error("Error parsing hsts ingress annotations, skipping all hsts annotations")
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

	if proxyBuffers, exists := ingEx.Ingress.Annotations["nginx.org/proxy-buffers"]; exists {
		ingCfg.ProxyBuffers = proxyBuffers
	}
	if proxyBufferSize, exists := ingEx.Ingress.Annotations["nginx.org/proxy-buffer-size"]; exists {
		ingCfg.ProxyBufferSize = proxyBufferSize
	}
	if proxyMaxTempFileSize, exists := ingEx.Ingress.Annotations["nginx.org/proxy-max-temp-file-size"]; exists {
		ingCfg.ProxyMaxTempFileSize = proxyMaxTempFileSize
	}
	return ingCfg
}

func getWebsocketServices(ingEx *IngressEx) map[string]bool {
	wsServices := make(map[string]bool)

	if services, exists := ingEx.Ingress.Annotations["nginx.org/websocket-services"]; exists {
		for _, svc := range strings.Split(services, ",") {
			wsServices[svc] = true
		}
	}

	return wsServices
}

func getRewrites(ingEx *IngressEx) (rewrites map[string]string, err error) {
	rewrites = make(map[string]string)

	if services, exists := ingEx.Ingress.Annotations["nginx.org/rewrites"]; exists {
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

func getSSLServices(ingEx *IngressEx) map[string]bool {
	sslServices := make(map[string]bool)

	if services, exists := ingEx.Ingress.Annotations["nginx.org/ssl-services"]; exists {
		for _, svc := range strings.Split(services, ",") {
			sslServices[svc] = true
		}
	}

	return sslServices
}

func createLocation(path string, upstream config.Upstream, cfg *config.Config, websocket bool, rewrite string, ssl bool) config.Location {
	loc := config.Location{
		Path:                 path,
		Upstream:             upstream,
		ProxyConnectTimeout:  cfg.ProxyConnectTimeout,
		ProxyReadTimeout:     cfg.ProxyReadTimeout,
		ClientMaxBodySize:    cfg.ClientMaxBodySize,
		Websocket:            websocket,
		Rewrite:              rewrite,
		SSL:                  ssl,
		ProxyBuffering:       cfg.ProxyBuffering,
		ProxyBuffers:         cfg.ProxyBuffers,
		ProxyBufferSize:      cfg.ProxyBufferSize,
		ProxyMaxTempFileSize: cfg.ProxyMaxTempFileSize,
		LocationSnippets:     cfg.LocationSnippets,
	}

	return loc
}

func (cnf *Configurator) createUpstream(ingEx *IngressEx, name string, backend *extensions.IngressBackend, namespace string) config.Upstream {
	ups := NewUpstreamWithDefaultServer(name)

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

func pathOrDefault(path string) string {
	if path == "" {
		return "/"
	}
	return path
}

func getNameForUpstream(ing *extensions.Ingress, host string, service string) string {
	return fmt.Sprintf("%v-%v-%v-%v", ing.Namespace, ing.Name, host, service)
}

func upstreamMapToSlice(upstreams map[string]config.Upstream) []config.Upstream {
	result := make([]config.Upstream, 0, len(upstreams))

	for _, ups := range upstreams {
		result = append(result, ups)
	}

	return result
}

// DeleteIngress deletes NGINX configuration for an Ingress resource
func (cnf *Configurator) DeleteIngress(name string) {
	cnf.lock.Lock()
	defer cnf.lock.Unlock()

	cnf.nginx.DeleteIngress(name)
	if err := cnf.nginx.Reload(); err != nil {
		log.Errorf("Error when removing ingress %q: %q", name, err)
	}
}

// UpdateEndpoints updates endpoints in NGINX configuration for an Ingress resource
func (cnf *Configurator) UpdateEndpoints(name string, ingEx *IngressEx) {
	cnf.AddOrUpdateIngress(name, ingEx)
}

// UpdateConfig updates NGINX Configuration parameters
func (cnf *Configurator) UpdateConfig(c *config.Config) {
	cnf.lock.Lock()
	defer cnf.lock.Unlock()

	cnf.config = c
	mainCfg := &config.MainConfig{
		HTTPSnippets:              c.MainHTTPSnippets,
		ServerNamesHashBucketSize: c.MainServerNamesHashBucketSize,
		ServerNamesHashMaxSize:    c.MainServerNamesHashMaxSize,
		LogFormat:                 c.MainLogFormat,
		SSLProtocols:              c.MainServerSSLProtocols,
		SSLCiphers:                c.MainServerSSLCiphers,
		SSLDHParam:                c.MainServerSSLDHParam,
		SSLPreferServerCiphers:    c.MainServerSSLPreferServerCiphers,
	}

	cnf.nginx.UpdateMainConfigFile(mainCfg)
}
