package config

import (
	"github.com/thetechnick/nginx-ingress/pkg/storage/pb"
	api_v1 "k8s.io/client-go/pkg/api/v1"
	extensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
)

// KeyFunc is used to get the k8s object key,
// which identifies an object
var KeyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc

// Upstream describes an NGINX upstream
type Upstream struct {
	Name            string
	UpstreamServers []UpstreamServer
}

// NewUpstreamWithDefaultServer creates an upstream with the default server.
// proxy_pass to an upstream with the default server returns 502.
// We use it for services that have no endpoints
func NewUpstreamWithDefaultServer(name string) Upstream {
	return Upstream{
		Name:            name,
		UpstreamServers: []UpstreamServer{UpstreamServer{Address: "127.0.0.1", Port: "8181"}},
	}
}

// UpstreamServer describes a server in an NGINX upstream
type UpstreamServer struct {
	Address string
	Port    string
}

// Server describes an NGINX server
type Server struct {
	Name              string
	Locations         []Location
	Upstreams         []Upstream
	SSL               bool
	SSLCertificate    string
	SSLCertificateKey string
	Files             []*pb.File

	// settings/annotations
	ServerSnippets        []string
	ServerTokens          bool
	HTTP2                 bool
	RedirectToHTTPS       bool
	ProxyProtocol         bool
	HSTS                  bool
	HSTSMaxAge            int64
	HSTSIncludeSubdomains bool
	ProxyHideHeaders      []string
	ProxyPassHeaders      []string

	// http://nginx.org/en/docs/http/ngx_http_realip_module.html
	RealIPHeader    string
	SetRealIPFrom   []string
	RealIPRecursive bool
}

// CreateServerConfig creates a new server config from the given params
func CreateServerConfig(gCfg *GlobalConfig, ingCfg *IngressConfig) *Server {
	return &Server{
		ServerTokens:          defaultBool(gCfg.ServerTokens, ingCfg.ServerTokens),
		HTTP2:                 defaultBool(gCfg.HTTP2, ingCfg.HTTP2),
		RedirectToHTTPS:       defaultBool(gCfg.RedirectToHTTPS, ingCfg.RedirectToHTTPS),
		ProxyProtocol:         defaultBool(gCfg.ProxyProtocol, ingCfg.ProxyProtocol),
		HSTS:                  defaultBool(gCfg.HSTS, ingCfg.HSTS),
		HSTSMaxAge:            defaultInt64(gCfg.HSTSMaxAge, ingCfg.HSTSMaxAge),
		HSTSIncludeSubdomains: defaultBool(gCfg.HSTSIncludeSubdomains, ingCfg.HSTSIncludeSubdomains),
		RealIPHeader:          defaultString(gCfg.RealIPHeader, ingCfg.RealIPHeader),
		SetRealIPFrom:         defaultStringSlice(gCfg.SetRealIPFrom, ingCfg.SetRealIPFrom),
		RealIPRecursive:       defaultBool(gCfg.RealIPRecursive, ingCfg.RealIPRecursive),
		ProxyHideHeaders:      defaultStringSlice(gCfg.ProxyHideHeaders, ingCfg.ProxyHideHeaders),
		ProxyPassHeaders:      defaultStringSlice(gCfg.ProxyPassHeaders, ingCfg.ProxyPassHeaders),
		ServerSnippets:        defaultStringSlice(gCfg.ServerSnippets, ingCfg.ServerSnippets),
		Files:                 []*pb.File{},
	}
}

// Location describes an NGINX location
type Location struct {
	LocationSnippets     []string
	Path                 string
	Upstream             Upstream
	ProxyConnectTimeout  string
	ProxyReadTimeout     string
	ClientMaxBodySize    string
	Websocket            bool
	Rewrite              string
	SSL                  bool
	ProxyBuffering       bool
	ProxyBuffers         string
	ProxyBufferSize      string
	ProxyMaxTempFileSize string
}

// IngressEx holds an Ingress along with Endpoints of the services
// that are referenced in this Ingress
type IngressEx struct {
	Ingress   *extensions.Ingress
	Endpoints map[string][]string       // indexed by service Name + port
	Secrets   map[string]*api_v1.Secret // indexed by secret name
}

// CreateLocation creates a new location from the given params
func CreateLocation(
	path string,
	upstream Upstream,
	gCfg *GlobalConfig,
	ingCfg *IngressConfig,
	websocket bool,
	rewrite string,
	ssl bool,
) Location {
	loc := Location{
		Path:      path,
		Upstream:  upstream,
		Websocket: websocket,
		Rewrite:   rewrite,
		SSL:       ssl,

		ProxyConnectTimeout:  defaultString(gCfg.ProxyConnectTimeout, ingCfg.ProxyConnectTimeout),
		ProxyReadTimeout:     defaultString(gCfg.ProxyReadTimeout, ingCfg.ProxyReadTimeout),
		ClientMaxBodySize:    defaultString(gCfg.ClientMaxBodySize, ingCfg.ClientMaxBodySize),
		ProxyBuffering:       defaultBool(gCfg.ProxyBuffering, ingCfg.ProxyBuffering),
		ProxyBuffers:         defaultString(gCfg.ProxyBuffers, ingCfg.ProxyBuffers),
		ProxyBufferSize:      defaultString(gCfg.ProxyBufferSize, ingCfg.ProxyBufferSize),
		ProxyMaxTempFileSize: defaultString(gCfg.ProxyMaxTempFileSize, ingCfg.ProxyMaxTempFileSize),
		LocationSnippets:     defaultStringSlice(gCfg.LocationSnippets, ingCfg.LocationSnippets),
	}

	return loc
}

func defaultString(d string, override *string) string {
	if override != nil {
		return *override
	}
	return d
}

func defaultBool(d bool, override *bool) bool {
	if override != nil {
		return *override
	}
	return d
}

func defaultStringSlice(d []string, override []string) []string {
	if override != nil {
		return override
	}
	return d
}

func defaultInt64(d int64, override *int64) int64 {
	if override != nil {
		return *override
	}
	return d
}
