package nginx

import (
	"testing"

	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/config"

	"github.com/stretchr/testify/assert"
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

	i := NewMergingCollisionHandler()
	t.Run("First ingress", func(t *testing.T) {
		assert := assert.New(t)
		result, err := i.AddConfigs(&ingress1, []config.Server{ingress1Server1})
		if assert.NoError(err) && assert.NotNil(result) && assert.Len(result, 1) {
			assert.Equal(result[0], ingress1Server1)
		}
	})

	t.Run("Merge 2nd ingress", func(t *testing.T) {
		assert := assert.New(t)
		result, err := i.AddConfigs(&ingress2, []config.Server{ingress2Server1, ingress2Server2})
		if assert.NoError(err) && assert.Len(result, 2, "Unexpected number of configs returned") {
			// 1. IngressNginxConfig
			assert.Equal(ingress2Server1.Name, result[0].Name, "Server names do not match")
			assert.Contains(result[0].Locations, ingress1Location1)
			assert.Contains(result[0].Locations, ingress2Location2)
			testServerHasDefaultSettings(assert, &result[0])
			if assert.Len(result[0].Upstreams, 2, "Unexpected number of upstreams") {
				assert.Contains(result[0].Upstreams, ingress1Upstream1)
				assert.Contains(result[0].Upstreams, ingress2Upstream1)
			}

			// 2. IngressNginxConfig
			assert.Equal(ingress2Server2.Name, result[1].Name, "Server names do not match")
			assert.Contains(result[1].Locations, ingress2Location3)
			testServerHasDefaultSettings(assert, &result[1])
			if assert.Len(result[1].Upstreams, 1, "Unexpected number of upstreams") {
				assert.Contains(result[1].Upstreams, ingress2Upstream2)
			}
		}
	})

	t.Run("Merge 3rd ingress", func(t *testing.T) {
		assert := assert.New(t)
		result, err := i.AddConfigs(&ingress3, []config.Server{ingress3Server1})
		if assert.NoError(err) && assert.Len(result, 1, "Unexpected number of configs returned") {
			// 1. IngressNginxConfig
			// The order of locations is not fixed, so we must use contains
			result := result[0]
			assert.Equal(ingress3Server1.Name, result.Name, "Server names do not match")
			testServerHasSettingsFromIngress3Server1(assert, &result)
			if assert.Len(result.Locations, 2, "Unexpected number of locations") {
				assert.Contains(result.Locations, ingress3Location1)
				assert.Contains(result.Locations, ingress2Location2)
			}
			if assert.Len(result.Upstreams, 2, "Unexpected number of upstreams") {
				assert.Contains(result.Upstreams, ingress3Upstream1)
				assert.Contains(result.Upstreams, ingress2Upstream1)
			}
		}
	})

	t.Run("Remove 2nd ingress", func(t *testing.T) {
		assert := assert.New(t)
		changed, deleted, err := i.RemoveConfigs("default/ing2")

		if assert.NoError(err) {
			if assert.Len(changed, 1, "Unexpected number of changed configs") {
				// 1. IngressNginxConfig
				assert.Equal(ingress1Server1.Name, changed[0].Name, "Server names do not match")
				testServerHasSettingsFromIngress3Server1(assert, &changed[0])
				if assert.Len(changed[0].Locations, 1) {
					assert.Equal(changed[0].Locations[0], ingress3Location1)
				}
				if assert.Len(changed[0].Upstreams, 1, "Unexpected number of upstreams") {
					assert.Equal(changed[0].Upstreams[0], ingress3Upstream1)
				}
			}
			if assert.Len(deleted, 1, "Unexpected number of deleted hosts") {
				assert.Contains(deleted, ingress2Server2)
			}
		}
	})

	t.Run("Try to remove 2nd ingress again", func(t *testing.T) {
		assert := assert.New(t)
		_, _, err := i.RemoveConfigs("default/ing2")
		if assert.Error(err) {
			assert.Equal("Ingress 'default/ing2' cannot be removed, because it was not found in the mapping", err.Error())
		}
	})
}
