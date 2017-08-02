package parser

import (
	"bytes"
	"errors"

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
		errs = append(errs, errors.New("missing certificate"))
	}
	key, ok := secret.Data[api_v1.TLSPrivateKeyKey]
	if !ok {
		errs = append(errs, errors.New("missing private key"))
	}

	if len(errs) > 0 {
		return nil, &ValidationError{secret, errs}
	}
	return bytes.Join([][]byte{cert, key}, []byte("\n")), nil
}
