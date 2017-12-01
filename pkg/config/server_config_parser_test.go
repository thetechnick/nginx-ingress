package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thetechnick/nginx-ingress/pkg/storage/pb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

func TestPathOrDefault(t *testing.T) {
	t.Run("Default", func(t *testing.T) {
		path := ""
		expected := "/"
		if pathOrDefault(path) != expected {
			t.Errorf("pathOrDefault(%q) should return %q", path, expected)
		}
	})

	t.Run("Else", func(t *testing.T) {
		path := "/path/to/resource"
		if pathOrDefault(path) != path {
			t.Errorf("pathOrDefault(%q) should return %q", path, path)
		}
	})
}

func TestServerConfigParser(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		assert := assert.New(t)
		p := NewServerConfigParser()

		ing := &v1beta1.Ingress{
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
		}
		ingCfg := IngressConfig{
			Ingress: ing,
		}

		servers, warning, err := p.Parse(
			*NewDefaultConfig(),
			ingCfg,
			map[string]*pb.File{
				"one.example.com": &pb.File{
					Name: "ssl/one.example.com",
				},
			},
			map[string][]string{
				"svc19000": []string{"8.8.8.8:9000"},
			},
		)

		if assert.NoError(err) && assert.NoError(warning) {
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
	})

	t.Run("No root path", func(t *testing.T) {
		assert := assert.New(t)
		p := NewServerConfigParser()
		ing := &v1beta1.Ingress{
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
										Path: "/test123",
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
		ingCfg := IngressConfig{
			Ingress: ing,
		}

		servers, warning, err := p.Parse(
			*NewDefaultConfig(),
			ingCfg,
			map[string]*pb.File{
				"one.example.com": &pb.File{
					Name: "ssl/one.example.com",
				},
			},
			map[string][]string{
				"svc19000": []string{"8.8.8.8:9000"},
			},
		)

		if assert.NoError(err) && assert.NoError(warning) {
			if assert.Len(servers, 1) {
				s := servers[0]

				if assert.Len(s.Locations, 1) &&
					assert.Len(s.Upstreams, 1) {
					assert.Equal("/test123", s.Locations[0].Path)
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
	})
}
