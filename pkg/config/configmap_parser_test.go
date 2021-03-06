package config

import (
	"fmt"
	"testing"

	"github.com/thetechnick/nginx-ingress/pkg/errors"

	"github.com/stretchr/testify/assert"

	api_v1 "k8s.io/client-go/pkg/api/v1"
)

func TestConfigMapKeyError(t *testing.T) {
	assert := assert.New(t)

	e := &ConfigMapKeyError{"test", fmt.Errorf("error")}
	assert.Equal("Skipping key \"test\": error", e.Error())
}

func TestConfigMapParser(t *testing.T) {
	p := NewConfigMapParser()

	t.Run("should emit no errors on empty configmap", func(t *testing.T) {
		assert := assert.New(t)
		c, err := p.Parse(&api_v1.ConfigMap{})

		assert.Nil(err, "A empty configmap should not produce errors")
		if assert.NotNil(c) {
			assert.Equal(NewDefaultConfig(), c, "Config should be equal to default config")
		}
	})

	t.Run("should not add any hsts setting if there is a error with one of the settings", func(t *testing.T) {
		assert := assert.New(t)

		c, err := p.Parse(&api_v1.ConfigMap{
			Data: map[string]string{
				"hsts":                    "not a bool",
				"hsts-max-age":            "123",
				"hsts-include-subdomains": "True",
			},
		})

		if assert.NotNil(err) && assert.Implements((*errors.ErrObjectContext)(nil), err) {
			cerr := err.(errors.ErrObjectContext)
			if assert.IsType(ValidationError{}, cerr.WrappedError()) {
				verr := cerr.WrappedError().(ValidationError)
				assert.Len(verr, 2)
			}
		}

		if assert.NotNil(c) {
			assert.Equal(NewDefaultConfig(), c, "Config should be equal to default config")
		}
	})

	t.Run("should return all errors present in the ConfigMap", func(t *testing.T) {
		assert := assert.New(t)

		c, err := p.Parse(&api_v1.ConfigMap{
			Data: map[string]string{
				"server-tokens":             "not a bool",
				"http2":                     "not a bool",
				"redirect-to-https":         "not a bool",
				"hsts":                      "not a bool",
				"hsts-max-age":              "not a int",
				"hsts-include-subdomains":   "not a bool",
				"proxy-protocol":            "not a bool",
				"real-ip-recursive":         "not a bool",
				"ssl-prefer-server-ciphers": "not a bool",
				"proxy-buffering":           "not a bool",
			},
		})

		if assert.NotNil(err) && assert.Implements((*errors.ErrObjectContext)(nil), err) {
			cerr := err.(errors.ErrObjectContext)
			if assert.IsType(ValidationError{}, cerr.WrappedError()) {
				verr := cerr.WrappedError().(ValidationError)
				assert.Len(verr, 11)
			}
		}

		if assert.NotNil(c) {
			assert.Equal(NewDefaultConfig(), c, "Config should be equal to default config")
		}
	})
}
