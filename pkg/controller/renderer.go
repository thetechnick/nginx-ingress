package controller

import (
	"bytes"
	"text/template"

	"github.com/thetechnick/nginx-ingress/pkg/collision"
	"github.com/thetechnick/nginx-ingress/pkg/config"
	"github.com/thetechnick/nginx-ingress/pkg/storage"
	"github.com/thetechnick/nginx-ingress/pkg/storage/pb"

	log "github.com/sirupsen/logrus"
)

// Renderer generates storage objects
type Renderer interface {
	RenderMainConfig(controllerConfig *config.Config) (*pb.MainConfig, error)
	RenderServerConfig(mergedConfig *collision.MergedIngressConfig) (*pb.ServerConfig, error)
}

type renderer struct {
	mainConfigTemplate *template.Template
	serverTemplate     *template.Template
}

// NewRenderer creates a new Renderer
func NewRenderer() Renderer {
	c := &renderer{}
	serverTemplate, err := template.New("ingress.tmpl").ParseFiles("ingress.tmpl")
	if err != nil {
		log.WithError(err).Fatal("Error parsing main config template")
	}
	c.serverTemplate = serverTemplate

	mainConfigTemplate, err := template.New("nginx.conf.tmpl").ParseFiles("nginx.conf.tmpl")
	if err != nil {
		log.WithError(err).Fatal("Error parsing server template")
	}
	c.mainConfigTemplate = mainConfigTemplate
	return c
}

func (c *renderer) RenderMainConfig(controllerConfig *config.Config) (*pb.MainConfig, error) {
	mainCfg := &config.MainConfig{
		HTTPSnippets:              controllerConfig.MainHTTPSnippets,
		ServerNamesHashBucketSize: controllerConfig.MainServerNamesHashBucketSize,
		ServerNamesHashMaxSize:    controllerConfig.MainServerNamesHashMaxSize,
		LogFormat:                 controllerConfig.MainLogFormat,
		SSLProtocols:              controllerConfig.MainServerSSLProtocols,
		SSLCiphers:                controllerConfig.MainServerSSLCiphers,
		SSLPreferServerCiphers:    controllerConfig.MainServerSSLPreferServerCiphers,
	}

	mc := &pb.MainConfig{}
	if controllerConfig.MainServerSSLDHParamFile != "" {
		mc.Dhparam = []byte(controllerConfig.MainServerSSLDHParamFile)
		mainCfg.SSLDHParam = storage.DHParamFile
	}

	var buffer bytes.Buffer
	err := c.mainConfigTemplate.Execute(&buffer, mainCfg)
	if err != nil {
		return nil, err
	}
	mc.Config = buffer.Bytes()
	return mc, nil
}

func (c *renderer) RenderServerConfig(mergedConfig *collision.MergedIngressConfig) (*pb.ServerConfig, error) {
	var buffer bytes.Buffer
	if err := c.serverTemplate.Execute(&buffer, mergedConfig.Server); err != nil {
		return nil, err
	}

	meta := map[string]string{}
	for _, ing := range mergedConfig.Ingress {
		ingKey, err := config.KeyFunc(ing)
		if err != nil {
			return nil, err
		}
		meta[ingKey] = ""
	}
	s := &pb.ServerConfig{
		Meta:   meta,
		Name:   mergedConfig.Server.Name,
		Config: buffer.Bytes(),
		Tls:    mergedConfig.Server.TLSCertificateFile,
	}
	return s, nil
}
