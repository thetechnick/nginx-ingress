package parser

import (
	"bytes"
	"fmt"

	"github.com/thetechnick/nginx-ingress/pkg/errors"
	api_v1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/record"
)

// SecretParser parses TLS secrets and validates them
type SecretParser interface {
	Parse(secret *api_v1.Secret) ([]byte, error)
}

// NewSecretParser returns a new SecretParser
func NewSecretParser() SecretParser {
	return &secretParser{}
}

type secretParser struct {
	recorder record.EventRecorder
}

func (p *secretParser) Parse(secret *api_v1.Secret) ([]byte, error) {
	errs := []error{}
	cert, ok := secret.Data[api_v1.TLSCertKey]
	if !ok {
		errs = append(errs, fmt.Errorf("missing certificate"))
	}
	key, ok := secret.Data[api_v1.TLSPrivateKeyKey]
	if !ok {
		errs = append(errs, fmt.Errorf("missing private key"))
	}

	if len(errs) > 0 {
		return nil, errors.WrapInObjectContext(ValidationError(errs), secret)
	}
	return bytes.Join([][]byte{cert, key}, []byte("\n")), nil
}
