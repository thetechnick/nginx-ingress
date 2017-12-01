package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thetechnick/nginx-ingress/pkg/errors"
	api_v1 "k8s.io/client-go/pkg/api/v1"
)

func TestSecretParser(t *testing.T) {
	p := NewSecretParser()

	t.Run("should return all errors", func(t *testing.T) {
		assert := assert.New(t)

		c, err := p.Parse(&api_v1.Secret{})

		if assert.NotNil(err) && assert.Implements((*errors.ErrObjectContext)(nil), err) {
			cerr := err.(errors.ErrObjectContext)
			verr := cerr.WrappedError().(ValidationError)
			assert.Len(verr, 2)
		}
		assert.Nil(c)
	})

	t.Run("should return the values", func(t *testing.T) {
		assert := assert.New(t)
		cert := []byte("cert")
		key := []byte("key")
		c, err := p.Parse(&api_v1.Secret{
			Data: map[string][]byte{
				api_v1.TLSCertKey:       cert,
				api_v1.TLSPrivateKeyKey: key,
			},
		})
		assert.Nil(err)
		if assert.NotNil(c) {
			assert.Equal([]byte("cert\nkey"), c)
		}
	})
}
