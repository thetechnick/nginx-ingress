package collision

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thetechnick/nginx-ingress/pkg/config"
)

func TestMergingCollisionHandler(t *testing.T) {
	testServerHasDefaultSettings := func(assert *assert.Assertions, server *config.Server) {
		assert.False(server.SSL, "SSL should not be active")
		assert.Empty(server.SSLCertificate, "SSLCertificate should be empty")
		assert.Empty(server.SSLCertificateKey, "SSLCertificateKey should be empty")
		assert.False(server.HTTP2, "HTTP2 should not be active")
		assert.False(server.HSTS, "HSTS should not be active")
		assert.Empty(server.HSTSMaxAge, "HSTSMaxAge should be empty")
		assert.False(server.HSTSIncludeSubdomains, "HSTSIncludeSubdomains should not be active")
	}

	testServerHasSettingsFromIngress3Server1 := func(assert *assert.Assertions, server *config.Server) {
		assert.True(server.SSL, "SSL should be active")
		assert.Equal("cert.pem", server.SSLCertificate, "SSLCertificate is not set")
		assert.Equal("cert.pem", server.SSLCertificateKey, "SSLCertificateKey is not set")
		assert.True(server.HTTP2, "HTTP2 should be active")
		assert.True(server.HSTS, "HSTS should be active")
		assert.Equal(int64(2000), server.HSTSMaxAge, "HSTSMaxAge is not set")
		assert.True(server.HSTSIncludeSubdomains, "HSTSIncludeSubdomains should be active")
	}

	t.Run("Single ingress", func(t *testing.T) {
		ch := NewMergingCollisionHandler()
		assert := assert.New(t)

		updated, err := ch.Resolve(MergeList{
			IngressConfig{
				&ingress1,
				[]*config.Server{&ingress1Server1},
			},
		})

		if assert.NoError(err) &&
			assert.NotNil(updated) {

			if assert.Len(updated, 1) {
				assert.Equal(&ingress1Server1, updated[0].Server)
				if assert.Len(updated[0].Ingress, 1) {
					assert.Equal(&ingress1, updated[0].Ingress[0])
				}
			}
		}
	})

	t.Run("Merge 1st and 2nd ingress", func(t *testing.T) {
		ch := NewMergingCollisionHandler()
		assert := assert.New(t)

		updated, err := ch.Resolve(MergeList{
			IngressConfig{
				&ingress1,
				[]*config.Server{&ingress1Server1},
			},
			IngressConfig{
				&ingress2,
				[]*config.Server{&ingress2Server1, &ingress2Server2},
			},
		})

		if assert.NoError(err) &&
			assert.NotNil(updated) {

			if assert.Len(updated, 2, "Unexpected number of merged configs") {
				// 1. MergedConfig
				config1 := updated[0]
				if assert.Len(config1.Ingress, 2) {
					assert.Contains(config1.Ingress, &ingress1)
					assert.Contains(config1.Ingress, &ingress2)
				}
				assert.Equal(ingress2Server1.Name, config1.Server.Name, "Server names do not match")
				assert.Contains(config1.Server.Locations, ingress1Location1)
				assert.Contains(config1.Server.Locations, ingress2Location2)
				testServerHasDefaultSettings(assert, config1.Server)
				if assert.Len(config1.Server.Upstreams, 2, "Unexpected number of upstreams") {
					assert.Contains(config1.Server.Upstreams, ingress1Upstream1)
					assert.Contains(config1.Server.Upstreams, ingress2Upstream1)
				}

				// 2. MergedConfig
				config2 := updated[1]
				if assert.Len(config2.Ingress, 1) {
					assert.Equal(&ingress2, config2.Ingress[0])
					assert.Equal(ingress2Server2.Name, config2.Server.Name, "Server names do not match")
					assert.Contains(config2.Server.Locations, ingress2Location3)
					testServerHasDefaultSettings(assert, config2.Server)
					if assert.Len(config2.Server.Upstreams, 1, "Unexpected number of upstreams") {
						assert.Contains(config2.Server.Upstreams, ingress2Upstream2)
					}
				}
			}
		}
	})

	t.Run("Merge 1st, 2nd and 3rd ingress", func(t *testing.T) {
		ch := NewMergingCollisionHandler()
		assert := assert.New(t)

		updated, err := ch.Resolve(MergeList{
			IngressConfig{
				&ingress1,
				[]*config.Server{&ingress1Server1},
			},
			IngressConfig{
				&ingress2,
				[]*config.Server{&ingress2Server1, &ingress2Server2},
			},
			IngressConfig{
				&ingress3,
				[]*config.Server{&ingress3Server1},
			},
		})

		if assert.NoError(err) &&
			assert.NotNil(updated) {

			if assert.Len(updated, 2, "Unexpected number of merged configs") {
				// 1. MergedConfig
				config1 := updated[0]
				if assert.Len(config1.Ingress, 3) {
					// Options are merged from all 3 ingress objects
					assert.Contains(config1.Ingress, &ingress1)
					assert.Contains(config1.Ingress, &ingress2)
					assert.Contains(config1.Ingress, &ingress3)
				}
				assert.Equal(ingress3Server1.Name, config1.Server.Name, "Server names do not match")
				assert.Contains(config1.Server.Locations, ingress2Location2)
				assert.Contains(config1.Server.Locations, ingress3Location1)
				testServerHasSettingsFromIngress3Server1(assert, config1.Server)
				if assert.Len(config1.Server.Upstreams, 2, "Unexpected number of upstreams") {
					assert.Contains(config1.Server.Upstreams, ingress3Upstream1)
					assert.Contains(config1.Server.Upstreams, ingress2Upstream1)
				}
			}
		}
	})
}
