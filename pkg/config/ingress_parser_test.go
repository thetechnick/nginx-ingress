package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

func TestIngParser(t *testing.T) {
	t.Run("No Annotations", func(t *testing.T) {
		assert := assert.New(t)
		p := NewIngressConfigParser()

		ing := &v1beta1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "ing1",
				Namespace:         "default",
				CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
				Annotations:       map[string]string{},
			},
			Spec: v1beta1.IngressSpec{
				Rules: []v1beta1.IngressRule{
					v1beta1.IngressRule{
						Host: "one.example.com",
						IngressRuleValue: v1beta1.IngressRuleValue{
							HTTP: &v1beta1.HTTPIngressRuleValue{
								Paths: []v1beta1.HTTPIngressPath{
									v1beta1.HTTPIngressPath{
										Path: "/",
										Backend: v1beta1.IngressBackend{
											ServiceName: "svc1",
											ServicePort: intstr.FromInt(9000),
										},
									},
								},
							},
						},
					},
				},
			},
		}
		ingCfg, warning, err := p.Parse(ing)

		if !assert.NoError(err) {
			return
		}
		assert.Nil(warning)
		assert.Equal(ing, ingCfg.Ingress)

		assert.Nil(ingCfg.LocationSnippets, "LocationSnippets")
		assert.Nil(ingCfg.ServerSnippets, "ServerSnippets")
		assert.Nil(ingCfg.ServerTokens, "ServerTokens")
		assert.Nil(ingCfg.ClientMaxBodySize, "ClientMaxBodySize")
		assert.Nil(ingCfg.RedirectToHTTPS, "RedirectToHTTPS")
		assert.Nil(ingCfg.ProxyBuffering, "ProxyBuffering")
		assert.Nil(ingCfg.ProxyConnectTimeout, "ProxyConnectTimeout")
		assert.Nil(ingCfg.ProxyReadTimeout, "ProxyReadTimeout")
		assert.Nil(ingCfg.ProxyBuffers, "ProxyBuffers")
		assert.Nil(ingCfg.ProxyBufferSize, "ProxyBufferSize")
		assert.Nil(ingCfg.ProxyMaxTempFileSize, "ProxyMaxTempFileSize")
		assert.Nil(ingCfg.ProxyProtocol, "ProxyProtocol")
		assert.Nil(ingCfg.ProxyHideHeaders, "ProxyHideHeaders")
		assert.Nil(ingCfg.ProxyPassHeaders, "ProxyPassHeaders")
		assert.Nil(ingCfg.ProxyReadTimeout, "ProxyReadTimeout")
		assert.Nil(ingCfg.HSTS, "HSTS")
		assert.Nil(ingCfg.HSTSMaxAge, "HSTSMaxAge")
		assert.Nil(ingCfg.HSTSIncludeSubdomains, "HSTSIncludeSubdomains")
		assert.Nil(ingCfg.RealIPHeader, "RealIPHeader")
		assert.Nil(ingCfg.SetRealIPFrom, "SetRealIPFrom")
		assert.Nil(ingCfg.RealIPRecursive, "RealIPRecursive")
		assert.Nil(ingCfg.LocationModifier, "LocationModifier")

		assert.NotNil(ingCfg.WebsocketServices, "WebsocketServices")
		assert.NotNil(ingCfg.Rewrites, "Rewrites")
		assert.NotNil(ingCfg.SSLServices, "SSLServices")
	})

	t.Run("invalid nginx.org/location-modifier annotation", func(t *testing.T) {
		ing := &v1beta1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "ing1",
				Namespace:         "default",
				CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
				Annotations: map[string]string{
					"nginx.org/location-modifier": "hans",
				},
			},
			Spec: v1beta1.IngressSpec{
				Rules: []v1beta1.IngressRule{
					v1beta1.IngressRule{
						Host: "one.example.com",
						IngressRuleValue: v1beta1.IngressRuleValue{
							HTTP: &v1beta1.HTTPIngressRuleValue{
								Paths: []v1beta1.HTTPIngressPath{
									v1beta1.HTTPIngressPath{
										Path: "/",
										Backend: v1beta1.IngressBackend{
											ServiceName: "svc1",
											ServicePort: intstr.FromInt(9000),
										},
									},
								},
							},
						},
					},
				},
			},
		}

		p := NewIngressConfigParser()
		ingCfg, warning, err := p.Parse(ing)
		if assert.NoError(t, err) {
			assert.NotNil(t, warning)
			assert.Contains(t, warning.Error(), `"nginx.org/location-modifier": 'hans' is no valid location modifier`)
			assert.Nil(t, ingCfg.LocationModifier, "LocationModifier")
		}
	})
}

func TestParseRewrites(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		serviceName := "coffee-svc"
		serviceNamePart := "serviceName=" + serviceName
		rewritePath := "/beans/"
		rewritePathPart := "rewrite=" + rewritePath
		rewriteService := serviceNamePart + " " + rewritePathPart

		serviceNameActual, rewritePathActual, err := parseRewrites(rewriteService)
		if serviceName != serviceNameActual || rewritePath != rewritePathActual || err != nil {
			t.Errorf("parseRewrites(%s) should return %q, %q, nil; got %q, %q, %v", rewriteService, serviceName, rewritePath, serviceNameActual, rewritePathActual, err)
		}
	})

	t.Run("InvalidFormat", func(t *testing.T) {
		rewriteService := "serviceNamecoffee-svc rewrite=/"
		_, _, err := parseRewrites(rewriteService)
		if err == nil {
			t.Errorf("parseRewrites(%s) should return error, got nil", rewriteService)
		}
	})
}
