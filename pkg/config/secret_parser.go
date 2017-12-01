package config

import (
	"bytes"
	"fmt"
	"path"

	"github.com/thetechnick/nginx-ingress/pkg/errors"
	"github.com/thetechnick/nginx-ingress/pkg/storage"
	"github.com/thetechnick/nginx-ingress/pkg/storage/pb"
	api_v1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/record"
)

// TLSSecretParser parses TLS secrets and validates them
type TLSSecretParser interface {
	Parse(secret *api_v1.Secret) ([]byte, error)
}

// NewTLSSecretParser returns a new SecretParser
func NewTLSSecretParser() TLSSecretParser {
	return &tlsSecretParser{}
}

type tlsSecretParser struct {
	recorder record.EventRecorder
}

func (p *tlsSecretParser) Parse(secret *api_v1.Secret) ([]byte, error) {
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

// BasicAuthUserSecretParser parses secrets that contain basic auth user-password data
type BasicAuthUserSecretParser interface {
	Parse(secret *api_v1.Secret) (*pb.File, error)
}

func NewBasicAuthUserSecretParser() BasicAuthUserSecretParser {
	return &basicAuthUserSecretParser{}
}

type basicAuthUserSecretParser struct{}

func (p *basicAuthUserSecretParser) Parse(secret *api_v1.Secret) (*pb.File, error) {
	errs := []error{}
	users, ok := secret.Data["users"]
	if !ok {
		errs = append(errs, fmt.Errorf("missing private key"))
	}

	if len(errs) > 0 {
		return nil, errors.WrapInObjectContext(ValidationError(errs), secret)
	}
	return &pb.File{
		Name:    path.Join(storage.AuthDir, fmt.Sprintf("%s-%s.auth", secret.Namespace, secret.Name)),
		Content: users,
	}, nil
}
