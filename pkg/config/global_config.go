package config

// GlobalConfig holds global NGINX configuration parameters
// that may be overriden by ingress annotations
type GlobalConfig struct {
	LocationSnippets              []string
	ServerSnippets                []string
	ServerTokens                  bool
	ProxyConnectTimeout           string
	ProxyReadTimeout              string
	ClientMaxBodySize             string
	HTTP2                         bool
	RedirectToHTTPS               bool
	MainHTTPSnippets              []string
	MainServerNamesHashBucketSize string
	MainServerNamesHashMaxSize    string
	MainLogFormat                 string
	MainWorkerShutdownTimeout     string
	ProxyBuffering                bool
	ProxyBuffers                  string
	ProxyBufferSize               string
	ProxyMaxTempFileSize          string
	ProxyProtocol                 bool
	ProxyHideHeaders              []string
	ProxyPassHeaders              []string
	HSTS                          bool
	HSTSMaxAge                    int64
	HSTSIncludeSubdomains         bool

	// http://nginx.org/en/docs/http/ngx_http_realip_module.html
	RealIPHeader    string
	SetRealIPFrom   []string
	RealIPRecursive bool

	// http://nginx.org/en/docs/http/ngx_http_ssl_module.html
	MainServerSSLProtocols           string
	MainServerSSLPreferServerCiphers bool
	MainServerSSLCiphers             string
	MainServerSSLDHParamFile         string
}

// NewDefaultConfig creates a Config with default values
func NewDefaultConfig() *GlobalConfig {
	return &GlobalConfig{
		ServerTokens:               true,
		ProxyConnectTimeout:        "60s",
		ProxyReadTimeout:           "60s",
		ClientMaxBodySize:          "1m",
		MainServerNamesHashMaxSize: "512",
		MainWorkerShutdownTimeout:  "10s",
		ProxyBuffering:             true,
		HSTSMaxAge:                 2592000,
	}
}
