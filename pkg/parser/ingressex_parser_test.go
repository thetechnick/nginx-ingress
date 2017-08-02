package parser

import (
	"testing"
	"time"

	"k8s.io/client-go/pkg/apis/extensions/v1beta1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/config"
	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/storage/pb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	api_v1 "k8s.io/client-go/pkg/api/v1"
)

type IngressParserMock struct {
	mock.Mock
}

func (m *IngressParserMock) Parse(ingCfg config.Config, ingEx *config.IngressEx, pems map[string]*pb.TLSCertificate) ([]*config.Server, error) {
	args := m.Called(ingCfg, ingEx, pems)
	return args.Get(0).([]*config.Server), args.Error(1)
}

type SecretParserMock struct {
	mock.Mock
}

func (m *SecretParserMock) Parse(secret *api_v1.Secret) ([]byte, error) {
	args := m.Called(secret)
	return args.Get(0).([]byte), args.Error(1)
}

func TestIngressExParser(t *testing.T) {
	ingress1 := v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "ing1",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
		},
		Spec: v1beta1.IngressSpec{
			TLS: []v1beta1.IngressTLS{
				v1beta1.IngressTLS{
					SecretName: "secret1",
					Hosts:      []string{"one.example.com"},
				},
			},
		},
	}

	t.Run("Parse", func(t *testing.T) {
		assert := assert.New(t)

		p := NewIngressExParser().(*ingressExParser)
		ingressParserMock := &IngressParserMock{}
		secretParserMock := &SecretParserMock{}
		p.ingressParser = ingressParserMock
		p.secretParser = secretParserMock

		secret := &api_v1.Secret{}
		ingEx := &config.IngressEx{
			Ingress: &ingress1,
			Secrets: map[string]*api_v1.Secret{
				"secret1": secret,
			},
		}
		mainConfig := config.Config{}
		tlsCerts := map[string]*pb.TLSCertificate{
			"one.example.com": &pb.TLSCertificate{
				Name:    "ssl/one.example.com.pem",
				Content: []byte{},
			},
		}
		secretParserMock.On("Parse", secret).Return([]byte{}, nil)
		ingressParserMock.On("Parse", mainConfig, ingEx, tlsCerts).Return([]*config.Server{}, nil)

		_, err := p.Parse(mainConfig, ingEx)
		assert.NoError(err)
		secretParserMock.AssertCalled(t, "Parse", secret)
		ingressParserMock.AssertCalled(t, "Parse", mainConfig, ingEx, tlsCerts)
	})

	t.Run("Parse wrap errors into IngressExValidationError", func(t *testing.T) {
		assert := assert.New(t)

		p := NewIngressExParser().(*ingressExParser)
		ingressParserMock := &IngressParserMock{}
		p.ingressParser = ingressParserMock
		p.secretParser = &SecretParserMock{}

		ingEx := &config.IngressEx{
			Ingress: &ingress1,
		}
		mainConfig := config.Config{}
		tlsCerts := map[string]*pb.TLSCertificate{}
		validationError := &ValidationError{&ingress1, []error{}}
		ingressParserMock.On("Parse", mainConfig, ingEx, tlsCerts).Return([]*config.Server{}, validationError)

		_, err := p.Parse(mainConfig, ingEx)
		if assert.Error(err) {
			assert.IsType(&IngressExValidationError{}, err)
			assert.Equal(validationError, err.(*IngressExValidationError).IngressError)
		}
	})
}
