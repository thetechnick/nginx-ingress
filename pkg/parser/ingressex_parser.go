package parser

import (
	"fmt"
	"path"

	"github.com/thetechnick/nginx-ingress/pkg/config"
	"github.com/thetechnick/nginx-ingress/pkg/storage"
	"github.com/thetechnick/nginx-ingress/pkg/storage/pb"
)

// IngressExValidationError contains Secret and Ingress validation errors
type IngressExValidationError struct {
	SecretErrors []*ValidationError
	IngressError *ValidationError
}

func (e *IngressExValidationError) Error() string {
	return fmt.Sprintf("ingress: %v, secret: %v", e.IngressError, e.SecretErrors)
}

// IngressExParser combines the secret parser and the ingress parser
type IngressExParser interface {
	Parse(config.Config, *config.IngressEx) ([]*config.Server, error)
}

// NewIngressExParser creates a new IngressExParser
func NewIngressExParser() IngressExParser {
	return &ingressExParser{
		secretParser:  NewSecretParser(),
		ingressParser: NewIngressParser(),
	}
}

type ingressExParser struct {
	secretParser  SecretParser
	ingressParser IngressParser
}

func (p *ingressExParser) Parse(mainConfig config.Config, ingEx *config.IngressEx) ([]*config.Server, error) {
	// TLS
	secretErrors := []*ValidationError{}
	tlsCerts := map[string]*pb.TLSCertificate{}
	for _, tls := range ingEx.Ingress.Spec.TLS {
		secretName := tls.SecretName
		secret, exist := ingEx.Secrets[secretName]
		if !exist {
			continue
		}

		tlsCert, err := p.secretParser.Parse(secret)
		if err != nil {
			if err, ok := err.(*ValidationError); ok {
				// just gather validation errors
				secretErrors = append(secretErrors, err)
			} else {
				return nil, err
			}
		}
		if tlsCert == nil {
			continue
		}

		for _, host := range tls.Hosts {
			tlsName := path.Join(storage.CertificatesDir, fmt.Sprintf("%s.pem", host))
			tlsCerts[host] = &pb.TLSCertificate{
				Name:    tlsName,
				Content: tlsCert,
			}
		}
		if len(tls.Hosts) == 0 {
			tlsName := path.Join(storage.CertificatesDir, "default.pem")
			tlsCerts[emptyHost] = &pb.TLSCertificate{
				Name:    tlsName,
				Content: tlsCert,
			}
		}
	}

	// Server
	generatedServers, err := p.ingressParser.Parse(mainConfig, ingEx, tlsCerts)
	if err != nil {
		if verr, ok := err.(*ValidationError); ok {
			// just gather validation errors
			return generatedServers, &IngressExValidationError{secretErrors, verr}
		}
		return nil, err
	}

	if len(secretErrors) > 0 {
		return generatedServers, &IngressExValidationError{secretErrors, nil}
	}
	return generatedServers, nil
}
