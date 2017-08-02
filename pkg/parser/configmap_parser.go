package parser

import (
	"errors"
	"fmt"
	"strings"

	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/config"
	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/util"
	api_v1 "k8s.io/client-go/pkg/api/v1"
)

// ConfigMapKeyError is a config error for a specific key of the ConfigMap
type ConfigMapKeyError struct {
	Key             string
	ValidationError error
}

func (e *ConfigMapKeyError) Error() string {
	return fmt.Sprintf("Skipping key %q: %v", e.Key, e.ValidationError)
}

// ConfigMapParser parses the server config from a ConfigMap
type ConfigMapParser interface {
	Parse(cfgm *api_v1.ConfigMap) (*config.Config, error)
}

// NewConfigMapParser returns a new ConfigMapParser
func NewConfigMapParser() ConfigMapParser {
	return &configMapParser{}
}

type configMapParser struct{}

func (p *configMapParser) Parse(cfgm *api_v1.ConfigMap) (*config.Config, error) {
	errs := []error{}
	cfg := config.NewDefaultConfig()

	// No validator/parser
	if clientMaxBodySize, exists := cfgm.Data["client-max-body-size"]; exists {
		cfg.ClientMaxBodySize = clientMaxBodySize
	}
	if serverNamesHashBucketSize, exists := cfgm.Data["server-names-hash-bucket-size"]; exists {
		cfg.MainServerNamesHashBucketSize = serverNamesHashBucketSize
	}
	if serverNamesHashMaxSize, exists := cfgm.Data["server-names-hash-max-size"]; exists {
		cfg.MainServerNamesHashMaxSize = serverNamesHashMaxSize
	}
	if proxyConnectTimeout, exists := cfgm.Data["proxy-connect-timeout"]; exists {
		cfg.ProxyConnectTimeout = proxyConnectTimeout
	}
	if proxyReadTimeout, exists := cfgm.Data["proxy-read-timeout"]; exists {
		cfg.ProxyReadTimeout = proxyReadTimeout
	}
	if realIPHeader, exists := cfgm.Data["real-ip-header"]; exists {
		cfg.RealIPHeader = realIPHeader
	}
	if sslProtocols, exists := cfgm.Data["ssl-protocols"]; exists {
		cfg.MainServerSSLProtocols = sslProtocols
	}
	if sslCiphers, exists := cfgm.Data["ssl-ciphers"]; exists {
		cfg.MainServerSSLCiphers = strings.Trim(sslCiphers, "\n")
	}
	if proxyBuffers, exists := cfgm.Data["proxy-buffers"]; exists {
		cfg.ProxyBuffers = proxyBuffers
	}
	if proxyBufferSize, exists := cfgm.Data["proxy-buffer-size"]; exists {
		cfg.ProxyBufferSize = proxyBufferSize
	}
	if proxyMaxTempFileSize, exists := cfgm.Data["proxy-max-temp-file-size"]; exists {
		cfg.ProxyMaxTempFileSize = proxyMaxTempFileSize
	}
	if mainHTTPSnippets, exists := util.GetMapKeyAsStringSlice(cfgm.Data, "http-snippets", cfgm, "\n"); exists {
		cfg.MainHTTPSnippets = mainHTTPSnippets
	}
	if locationSnippets, exists := util.GetMapKeyAsStringSlice(cfgm.Data, "location-snippets", cfgm, "\n"); exists {
		cfg.LocationSnippets = locationSnippets
	}
	if serverSnippets, exists := util.GetMapKeyAsStringSlice(cfgm.Data, "server-snippets", cfgm, "\n"); exists {
		cfg.ServerSnippets = serverSnippets
	}
	if proxyHideHeaders, exists := util.GetMapKeyAsStringSlice(cfgm.Data, "proxy-hide-headers", cfgm, ","); exists {
		cfg.ProxyHideHeaders = proxyHideHeaders
	}
	if proxyPassHeaders, exists := util.GetMapKeyAsStringSlice(cfgm.Data, "proxy-pass-headers", cfgm, ","); exists {
		cfg.ProxyPassHeaders = proxyPassHeaders
	}
	if sslDHParamFile, exists := cfgm.Data["ssl-dhparam-file"]; exists {
		cfg.MainServerSSLDHParamFile = strings.Trim(sslDHParamFile, "\n")
	}
	if logFormat, exists := cfgm.Data["log-format"]; exists {
		cfg.MainLogFormat = logFormat
	}
	if setRealIPFrom, exists := util.GetMapKeyAsStringSlice(cfgm.Data, "set-real-ip-from", cfgm, ","); exists {
		cfg.SetRealIPFrom = setRealIPFrom
	}

	//
	// Validated keys
	if serverTokens, exists, err := util.GetMapKeyAsBool(cfgm.Data, "server-tokens"); exists {
		if err != nil {
			errs = append(errs, &ConfigMapKeyError{"server-tokens", err})
		} else {
			cfg.ServerTokens = serverTokens
		}
	}

	if HTTP2, exists, err := util.GetMapKeyAsBool(cfgm.Data, "http2"); exists {
		if err != nil {
			errs = append(errs, &ConfigMapKeyError{"http2", err})
		} else {
			cfg.HTTP2 = HTTP2
		}
	}
	if redirectToHTTPS, exists, err := util.GetMapKeyAsBool(cfgm.Data, "redirect-to-https"); exists {
		if err != nil {
			errs = append(errs, &ConfigMapKeyError{"redirect-to-https", err})
		} else {
			cfg.RedirectToHTTPS = redirectToHTTPS
		}
	}

	if hsts, exists, err := util.GetMapKeyAsBool(cfgm.Data, "hsts"); exists {
		parsingErrors := false
		if err != nil {
			errs = append(errs, &ConfigMapKeyError{"hsts", err})
			parsingErrors = true
		}

		hstsMaxAge, existsMA, err := util.GetMapKeyAsInt(cfgm.Data, "hsts-max-age")
		if existsMA && err != nil {
			errs = append(errs, &ConfigMapKeyError{"hsts-max-age", err})
			parsingErrors = true
		}
		hstsIncludeSubdomains, existsIS, err := util.GetMapKeyAsBool(cfgm.Data, "hsts-include-subdomains")
		if existsIS && err != nil {
			errs = append(errs, &ConfigMapKeyError{"hsts-include-subdomains", err})
			parsingErrors = true
		}

		if parsingErrors {
			errs = append(errs, errors.New("Error validating HSTS settings in ConfigMap, skipping keys for all hsts settings"))
		} else {
			cfg.HSTS = hsts
			if existsMA {
				cfg.HSTSMaxAge = hstsMaxAge
			}
			if existsIS {
				cfg.HSTSIncludeSubdomains = hstsIncludeSubdomains
			}
		}
	}

	if proxyProtocol, exists, err := util.GetMapKeyAsBool(cfgm.Data, "proxy-protocol"); exists {
		if err != nil {
			errs = append(errs, &ConfigMapKeyError{"proxy-protocol", err})
		} else {
			cfg.ProxyProtocol = proxyProtocol
		}
	}

	if realIPRecursive, exists, err := util.GetMapKeyAsBool(cfgm.Data, "real-ip-recursive"); exists {
		if err != nil {
			errs = append(errs, &ConfigMapKeyError{"real-ip-recursive", err})
		} else {
			cfg.RealIPRecursive = realIPRecursive
		}
	}

	if sslPreferServerCiphers, exists, err := util.GetMapKeyAsBool(cfgm.Data, "ssl-prefer-server-ciphers"); exists {
		if err != nil {
			errs = append(errs, &ConfigMapKeyError{"ssl-prefer-server-ciphers", err})
		} else {
			cfg.MainServerSSLPreferServerCiphers = sslPreferServerCiphers
		}
	}

	if proxyBuffering, exists, err := util.GetMapKeyAsBool(cfgm.Data, "proxy-buffering"); exists {
		if err != nil {
			errs = append(errs, &ConfigMapKeyError{"proxy-buffering", err})
		} else {
			cfg.ProxyBuffering = proxyBuffering
		}
	}

	if len(errs) > 0 {
		return cfg, &ValidationError{cfgm, errs}
	}

	return cfg, nil
}
