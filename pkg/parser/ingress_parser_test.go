package parser

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/thetechnick/nginx-ingress/pkg/config"
	"github.com/thetechnick/nginx-ingress/pkg/storage/pb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

func TestIngressParser(t *testing.T) {
	assert := assert.New(t)
	p := NewIngressParser()

	servers, err := p.Parse(*config.NewDefaultConfig(), &config.IngressEx{
		Ingress: &v1beta1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "ing1",
				Namespace:         "default",
				CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
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
		},
		Endpoints: map[string][]string{
			"svc19000": []string{"8.8.8.8:9000"},
		},
	}, map[string]*pb.TLSCertificate{
		"one.example.com": &pb.TLSCertificate{
			Name: "ssl/one.example.com",
		},
	})

	if assert.NoError(err) {
		if assert.Len(servers, 1) {
			s := servers[0]

			if assert.Len(s.Locations, 1) &&
				assert.Len(s.Upstreams, 1) {
				assert.Equal("/", s.Locations[0].Path)
				assert.Equal(s.Upstreams[0], s.Locations[0].Upstream)

				up := s.Upstreams[0]
				assert.Equal("default-ing1-one.example.com-svc1", up.Name)
				if assert.Len(up.UpstreamServers, 1) {
					assert.Equal("8.8.8.8", up.UpstreamServers[0].Address)
					assert.Equal("9000", up.UpstreamServers[0].Port)
				}
			}
			assert.True(s.SSL, "SSL should be enabled")
		}
	}
}

func TestPathOrDefaultReturnDefault(t *testing.T) {
	path := ""
	expected := "/"
	if pathOrDefault(path) != expected {
		t.Errorf("pathOrDefault(%q) should return %q", path, expected)
	}
}

func TestPathOrDefaultReturnActual(t *testing.T) {
	path := "/path/to/resource"
	if pathOrDefault(path) != path {
		t.Errorf("pathOrDefault(%q) should return %q", path, path)
	}
}

func TestParseRewrites(t *testing.T) {
	serviceName := "coffee-svc"
	serviceNamePart := "serviceName=" + serviceName
	rewritePath := "/beans/"
	rewritePathPart := "rewrite=" + rewritePath
	rewriteService := serviceNamePart + " " + rewritePathPart

	serviceNameActual, rewritePathActual, err := parseRewrites(rewriteService)
	if serviceName != serviceNameActual || rewritePath != rewritePathActual || err != nil {
		t.Errorf("parseRewrites(%s) should return %q, %q, nil; got %q, %q, %v", rewriteService, serviceName, rewritePath, serviceNameActual, rewritePathActual, err)
	}
}

func TestParseRewritesInvalidFormat(t *testing.T) {
	rewriteService := "serviceNamecoffee-svc rewrite=/"

	_, _, err := parseRewrites(rewriteService)
	if err == nil {
		t.Errorf("parseRewrites(%s) should return error, got nil", rewriteService)
	}
}
