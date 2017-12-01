package renderer

import (
	"github.com/thetechnick/nginx-ingress/pkg/config"
	"github.com/thetechnick/nginx-ingress/pkg/storage"
	"github.com/thetechnick/nginx-ingress/pkg/storage/pb"
)

// MainConfigTemplateData contains all values to render the main
// NGINX configuration file from its template "nginx.conf.tmpl"
type MainConfigTemplateData struct {
	ServerNamesHashBucketSize string
	ServerNamesHashMaxSize    string
	LogFormat                 string
	HealthStatus              bool
	HTTPSnippets              []string

	// http://nginx.org/en/docs/http/ngx_http_ssl_module.html
	SSLProtocols           string
	SSLPreferServerCiphers bool
	SSLCiphers             string
	SSLDHParamsFile        *pb.File

	WorkerShutdownTimeout string
}

func MainConfigTemplateDataFromIngressConfig(config *config.GlobalConfig) *MainConfigTemplateData {
	mainCfg := &MainConfigTemplateData{
		HTTPSnippets:              config.MainHTTPSnippets,
		ServerNamesHashBucketSize: config.MainServerNamesHashBucketSize,
		ServerNamesHashMaxSize:    config.MainServerNamesHashMaxSize,
		LogFormat:                 config.MainLogFormat,
		SSLProtocols:              config.MainServerSSLProtocols,
		SSLCiphers:                config.MainServerSSLCiphers,
		SSLPreferServerCiphers:    config.MainServerSSLPreferServerCiphers,
		WorkerShutdownTimeout:     config.MainWorkerShutdownTimeout,
	}

	if config.MainServerSSLDHParamFile != "" {
		mainCfg.SSLDHParamsFile = &pb.File{
			Name:    storage.DHParamFile,
			Content: []byte(config.MainServerSSLDHParamFile),
		}
	}

	return mainCfg
}
