package config

import (
	"fmt"
	"strings"

	"github.com/thetechnick/nginx-ingress/pkg/errors"
	"github.com/thetechnick/nginx-ingress/pkg/util"

	extensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

type IngressConfigParser interface {
	Parse(ingress *extensions.Ingress) (ingCfg *IngressConfig, warning error, err error)
}

func NewIngressConfigParser() IngressConfigParser {
	return &ingParser{}
}

type ingParser struct{}

func (p *ingParser) Parse(ing *extensions.Ingress) (ingCfg *IngressConfig, warning error, fErr error) {
	warnings := []error{}
	ingCfg = &IngressConfig{
		Ingress: ing,
	}

	if serverTokens, exists, err := util.GetMapKeyAsBool(ing.Annotations, "nginx.org/server-tokens"); exists {
		if err != nil {
			warnings = append(warnings, &IngressAnnotationError{"nginx.org/server-tokens", err})
		} else {
			ingCfg.ServerTokens = &serverTokens
		}
	}

	if serverSnippets, exists := util.GetMapKeyAsStringSlice(ing.Annotations, "nginx.org/server-snippets", ing, "\n"); exists {
		ingCfg.ServerSnippets = serverSnippets
	}
	if locationSnippets, exists := util.GetMapKeyAsStringSlice(ing.Annotations, "nginx.org/location-snippets", ing, "\n"); exists {
		ingCfg.LocationSnippets = locationSnippets
	}

	if proxyConnectTimeout, exists := ing.Annotations["nginx.org/proxy-connect-timeout"]; exists {
		ingCfg.ProxyConnectTimeout = &proxyConnectTimeout
	}
	if proxyReadTimeout, exists := ing.Annotations["nginx.org/proxy-read-timeout"]; exists {
		ingCfg.ProxyReadTimeout = &proxyReadTimeout
	}
	if proxyHideHeaders, exists := util.GetMapKeyAsStringSlice(ing.Annotations, "nginx.org/proxy-hide-headers", ing, ","); exists {
		ingCfg.ProxyHideHeaders = proxyHideHeaders
	}
	if proxyPassHeaders, exists := util.GetMapKeyAsStringSlice(ing.Annotations, "nginx.org/proxy-pass-headers", ing, ","); exists {
		ingCfg.ProxyPassHeaders = proxyPassHeaders
	}
	if clientMaxBodySize, exists := ing.Annotations["nginx.org/client-max-body-size"]; exists {
		ingCfg.ClientMaxBodySize = &clientMaxBodySize
	}
	if HTTP2, exists, err := util.GetMapKeyAsBool(ing.Annotations, "nginx.org/http2"); exists {
		if err != nil {
			warnings = append(warnings, &IngressAnnotationError{"nginx.org/http2", err})
		} else {
			ingCfg.HTTP2 = &HTTP2
		}
	}
	if redirectToHTTPS, exists, err := util.GetMapKeyAsBool(ing.Annotations, "nginx.org/redirect-to-https"); exists {
		if err != nil {
			warnings = append(warnings, &IngressAnnotationError{"nginx.org/redirect-to-https", err})
		} else {
			ingCfg.RedirectToHTTPS = &redirectToHTTPS
		}
	}
	if proxyBuffering, exists, err := util.GetMapKeyAsBool(ing.Annotations, "nginx.org/proxy-buffering"); exists {
		if err != nil {
			warnings = append(warnings, &IngressAnnotationError{"nginx.org/proxy-buffering", err})
		} else {
			ingCfg.ProxyBuffering = &proxyBuffering
		}
	}

	if hsts, exists, err := util.GetMapKeyAsBool(ing.Annotations, "nginx.org/hsts"); exists {
		if err != nil {
			warnings = append(warnings, &IngressAnnotationError{"nginx.org/hsts", err})
		} else {
			parsingErrors := false

			hstsMaxAge, existsMA, err := util.GetMapKeyAsInt(ing.Annotations, "nginx.org/hsts-max-age")
			if existsMA && err != nil {
				warnings = append(warnings, &IngressAnnotationError{"nginx.org/hsts-max-age", err})
				parsingErrors = true
			}
			hstsIncludeSubdomains, existsIS, err := util.GetMapKeyAsBool(ing.Annotations, "nginx.org/hsts-include-subdomains")
			if existsIS && err != nil {
				warnings = append(warnings, &IngressAnnotationError{"nginx.org/hsts-include-subdomains", err})
				parsingErrors = true
			}

			if parsingErrors {
				warnings = append(warnings, fmt.Errorf("Error parsing hsts ingress annotations, skipping all hsts annotations"))
			} else {
				ingCfg.HSTS = &hsts
				if existsMA {
					ingCfg.HSTSMaxAge = &hstsMaxAge
				}
				if existsIS {
					ingCfg.HSTSIncludeSubdomains = &hstsIncludeSubdomains
				}
			}
		}
	}

	if proxyBuffers, exists := ing.Annotations["nginx.org/proxy-buffers"]; exists {
		ingCfg.ProxyBuffers = &proxyBuffers
	}
	if proxyBufferSize, exists := ing.Annotations["nginx.org/proxy-buffer-size"]; exists {
		ingCfg.ProxyBufferSize = &proxyBufferSize
	}
	if proxyMaxTempFileSize, exists := ing.Annotations["nginx.org/proxy-max-temp-file-size"]; exists {
		ingCfg.ProxyMaxTempFileSize = &proxyMaxTempFileSize
	}

	ingCfg.WebsocketServices = getWebsocketServices(ing)
	ingCfg.SSLServices = getSSLServices(ing)
	rewrites, rerr := getRewrites(ing)
	ingCfg.Rewrites = rewrites
	if rerr != nil {
		warnings = append(warnings, &IngressAnnotationError{"nginx.org/rewrites", rerr})
	}

	if len(warnings) > 0 {
		warning = errors.WrapInObjectContext(ValidationError(warnings), ing)
	}
	return
}

// IngressConfig contains all configuration options from the ingress resource
type IngressConfig struct {
	Ingress *extensions.Ingress

	//
	// Annotations
	//
	LocationSnippets  []string
	ServerSnippets    []string
	ServerTokens      *bool
	ClientMaxBodySize *string
	HTTP2             *bool
	RedirectToHTTPS   *bool

	ProxyBuffering *bool
	ProxyConnectTimeout,
	ProxyReadTimeout,
	ProxyBuffers,
	ProxyBufferSize,
	ProxyMaxTempFileSize *string
	ProxyProtocol    *bool
	ProxyHideHeaders []string
	ProxyPassHeaders []string

	HSTS                  *bool
	HSTSMaxAge            *int64
	HSTSIncludeSubdomains *bool

	// http://nginx.org/en/docs/http/ngx_http_realip_module.html
	RealIPHeader    *string
	SetRealIPFrom   []string
	RealIPRecursive *bool

	WebsocketServices map[string]bool
	Rewrites          map[string]string
	SSLServices       map[string]bool
}

func getWebsocketServices(ing *extensions.Ingress) (wsServices map[string]bool) {
	wsServices = make(map[string]bool)
	if services, exists := ing.Annotations["nginx.org/websocket-services"]; exists {
		for _, svc := range strings.Split(services, ",") {
			wsServices[svc] = true
		}
	}
	return
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

func getSSLServices(ing *extensions.Ingress) (sslServices map[string]bool) {
	sslServices = make(map[string]bool)
	if services, exists := ing.Annotations["nginx.org/ssl-services"]; exists {
		for _, svc := range strings.Split(services, ",") {
			sslServices[svc] = true
		}
	}
	return
}
