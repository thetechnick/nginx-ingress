package controller

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/collision"
	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

func TestRenderer(t *testing.T) {
	t.Run("RenderMainConfig", func(t *testing.T) {
		c := NewRenderer()
		assert := assert.New(t)
		dhparamFile := "dhparam"

		config := config.NewDefaultConfig()
		config.MainServerSSLDHParamFile = dhparamFile

		mc, err := c.RenderMainConfig(config)
		assert.NoError(err)
		assert.Equal(dhparamFile, string(mc.Dhparam))
	})

	t.Run("RenderServerConfig", func(t *testing.T) {
		c := NewRenderer()
		assert := assert.New(t)

		name := "one.example.com"
		mc := &collision.MergedIngressConfig{
			Ingress: []*v1beta1.Ingress{
				&v1beta1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "ing1",
						Namespace:         "default",
						CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
					},
				},
			},
			Server: &config.Server{
				Name:              name,
				SSL:               true,
				SSLCertificate:    "cert.pem",
				SSLCertificateKey: "key.pem",
			},
		}
		sc, err := c.RenderServerConfig(mc)
		if assert.NoError(err) {
			if assert.Len(sc.Meta, 1) {
				_, ok := sc.Meta["default/ing1"]
				assert.True(ok, "ServerConfig metadata not set")
			}
			assert.Equal(name, sc.Name)

			config := string(sc.Config)
			assert.Regexp("listen 80;", config)
			assert.Regexp("listen 443 ssl;", config)
			assert.Regexp("ssl_certificate cert.pem;", config)
			assert.Regexp("ssl_certificate_key key.pem;", config)
			assert.Regexp("server_name one.example.com;", config)
		}
	})
}
