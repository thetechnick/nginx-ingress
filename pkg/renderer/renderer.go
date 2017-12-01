package renderer

import (
	"bytes"
	"text/template"

	"github.com/thetechnick/nginx-ingress/pkg/collision"
	"github.com/thetechnick/nginx-ingress/pkg/config"
	"github.com/thetechnick/nginx-ingress/pkg/storage/pb"

	log "github.com/sirupsen/logrus"
)

// Renderer generates storage objects
type Renderer interface {
	RenderMainConfig(mainConfig *MainConfigTemplateData) (*pb.MainConfig, error)
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

func (c *renderer) RenderMainConfig(mainCfg *MainConfigTemplateData) (*pb.MainConfig, error) {
	mc := &pb.MainConfig{}
	if mainCfg.SSLDHParamsFile != nil {
		mc.Files = []*pb.File{
			mainCfg.SSLDHParamsFile,
		}
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
		Files:  mergedConfig.Server.Files,
	}
	return s, nil
}
