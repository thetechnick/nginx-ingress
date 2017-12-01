package controller

import (
	"fmt"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/thetechnick/nginx-ingress/pkg/collision"
	"github.com/thetechnick/nginx-ingress/pkg/config"
	"github.com/thetechnick/nginx-ingress/pkg/errors"
	"github.com/thetechnick/nginx-ingress/pkg/parser"
	"github.com/thetechnick/nginx-ingress/pkg/storage/pb"
	"github.com/thetechnick/nginx-ingress/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	api_v1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

type IngressExParserMock struct {
	mock.Mock
}

func (m *IngressExParserMock) Parse(c config.Config, ingEx *config.IngressEx) ([]*config.Server, error) {
	args := m.Called(c, ingEx)
	return args.Get(0).([]*config.Server), args.Error(1)
}

type ConfigMapParserMock struct {
	mock.Mock
}

func (m *ConfigMapParserMock) Parse(cfgm *api_v1.ConfigMap) (*config.Config, error) {
	args := m.Called(cfgm)
	return args.Get(0).(*config.Config), args.Error(1)
}

type CollisionHandlerMock struct {
	mock.Mock
}

func (m *CollisionHandlerMock) Resolve(mergeList collision.MergeList) (updated []collision.MergedIngressConfig, err error) {
	args := m.Called(mergeList)
	return args.Get(0).([]collision.MergedIngressConfig), args.Error(1)
}

type IngressExStoreMock struct {
	mock.Mock
}

func (m *IngressExStoreMock) GetIngressEx(ingKey string) (*config.IngressEx, error) {
	args := m.Called(ingKey)
	return args.Get(0).(*config.IngressEx), args.Error(1)
}

type RendererMock struct {
	mock.Mock
}

func (m *RendererMock) RenderMainConfig(controllerConfig *config.Config) (*pb.MainConfig, error) {
	args := m.Called(controllerConfig)
	return args.Get(0).(*pb.MainConfig), args.Error(1)
}

func (m *RendererMock) RenderServerConfig(mergedConfig *collision.MergedIngressConfig) (*pb.ServerConfig, error) {
	args := m.Called(mergedConfig)
	return args.Get(0).(*pb.ServerConfig), args.Error(1)
}

type RecorderMock struct {
	mock.Mock
}

func (m *RecorderMock) Event(object runtime.Object, eventtype, reason, message string) {
	m.Called(object, eventtype, reason, message)
}

func (m *RecorderMock) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	m.Called(object, eventtype, reason, messageFmt, args)
}

func (m *RecorderMock) PastEventf(object runtime.Object, timestamp metav1.Time, eventtype, reason, messageFmt string, args ...interface{}) {
	m.Called(object, timestamp, eventtype, reason, messageFmt, args)
}

func TestConfigurator(t *testing.T) {
	var serverConfigStorage *test.ServerConfigStorageMock
	var mainConfigStorage *test.MainConfigStorageMock
	var ingressExParser *IngressExParserMock
	var configMapParser *ConfigMapParserMock
	var collisionHandler *CollisionHandlerMock
	var ingressExStore *IngressExStoreMock
	var renderer *RendererMock
	var recorder *RecorderMock
	var c *configurator

	ingress1 := v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "ing1",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
		},
	}
	ingEx1 := config.IngressEx{
		Ingress:   &ingress1,
		Secrets:   map[string]*api_v1.Secret{},
		Endpoints: map[string][]string{},
	}

	ingress2 := v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "ing2",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now()),
		},
	}
	ingEx2 := config.IngressEx{
		Ingress:   &ingress2,
		Secrets:   map[string]*api_v1.Secret{},
		Endpoints: map[string][]string{},
	}

	cfgm := api_v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "cfgm1",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
		},
		Data: map[string]string{},
	}

	beforeEach := func() {
		serverConfigStorage = &test.ServerConfigStorageMock{}
		mainConfigStorage = &test.MainConfigStorageMock{}
		ingressExParser = &IngressExParserMock{}
		configMapParser = &ConfigMapParserMock{}
		collisionHandler = &CollisionHandlerMock{}
		ingressExStore = &IngressExStoreMock{}
		renderer = &RendererMock{}
		recorder = &RecorderMock{}
		logger := log.New()
		logger.SetLevel(log.DebugLevel)

		c = &configurator{
			scs:             serverConfigStorage,
			mcs:             mainConfigStorage,
			ingExParser:     ingressExParser,
			configMapParser: configMapParser,
			ch:              collisionHandler,
			ingExStore:      ingressExStore,
			configurator:    renderer,
			recorder:        recorder,
			log:             logger.WithField("test", "TestConfigurator"),
		}
	}

	t.Run("IngressUpdated does nothing when the main config is missing", func(t *testing.T) {
		beforeEach()
		assert := assert.New(t)

		err := c.IngressUpdated("default/ing1")
		assert.NoError(err)
	})

	t.Run("IngressUpdated", func(t *testing.T) {
		beforeEach()
		assert := assert.New(t)

		c.mainConfig = config.NewDefaultConfig()
		server1 := &config.Server{
			Name: "one.example.com",
		}
		servers := []*config.Server{server1}
		mergeList := collision.MergeList{
			collision.IngressConfig{
				Ingress: ingEx1.Ingress,
				Servers: servers,
			},
			collision.IngressConfig{
				Ingress: ingEx2.Ingress,
				Servers: servers,
			},
		}
		mergedList := []collision.MergedIngressConfig{
			collision.MergedIngressConfig{
				Server:  server1,
				Ingress: []*v1beta1.Ingress{ingEx1.Ingress, ingEx2.Ingress},
			},
		}
		sc1 := &pb.ServerConfig{
			Meta: map[string]string{
				"default/ing2": "",
			},
			Name: "one.example.com",
		}
		sc2 := &pb.ServerConfig{
			Meta: map[string]string{
				"default/ing2": "",
			},
			Name: "two.example.com",
		}
		scl := []*pb.ServerConfig{
			sc1, sc2,
		}
		rendered := &pb.ServerConfig{
			Name: "one.example.com",
		}

		ingressExStore.On("GetIngressEx", "default/ing1").Return(&ingEx1, nil)
		ingressExStore.On("GetIngressEx", "default/ing2").Return(&ingEx2, nil)
		ingressExParser.On("Parse", *c.mainConfig, &ingEx1).Return(servers, nil)
		ingressExParser.On("Parse", *c.mainConfig, &ingEx2).Return(servers, nil)
		serverConfigStorage.On("Get", "one.example.com").Return(sc1, nil)
		serverConfigStorage.On("List").Return(scl, nil)
		serverConfigStorage.On("ByIngressKey", "default/ing1").Return([]*pb.ServerConfig{}, nil)
		serverConfigStorage.On("ByIngressKey", "default/ing2").Return(scl, nil)
		collisionHandler.On("Resolve", mergeList).Return(mergedList, nil)
		serverConfigStorage.On("Delete", sc2).Return(nil)
		renderer.On("RenderServerConfig", &mergedList[0]).Return(rendered, nil)
		serverConfigStorage.On("Put", rendered).Return(nil)
		renderer.On("RenderMainConfig", mock.Anything).Return(nil, nil)

		err := c.IngressUpdated("default/ing1")
		assert.NoError(err)
		serverConfigStorage.AssertCalled(t, "Delete", sc2)
		renderer.AssertCalled(t, "RenderServerConfig", &mergedList[0])
		serverConfigStorage.AssertCalled(t, "Put", rendered)
	})

	// ConfigUpdated
	t.Run("ConfigUpdated", func(t *testing.T) {
		beforeEach()
		assert := assert.New(t)

		nc := config.NewDefaultConfig()
		mc := &pb.MainConfig{}
		configMapParser.On("Parse", &cfgm).Return(nc, nil)
		renderer.On("RenderMainConfig", nc).Return(mc, nil)
		mainConfigStorage.On("Put", mc).Return(nil)

		err := c.ConfigUpdated(&cfgm)
		assert.NoError(err)
		configMapParser.AssertCalled(t, "Parse", &cfgm)
		renderer.AssertCalled(t, "RenderMainConfig", nc)
		mainConfigStorage.AssertCalled(t, "Put", mc)
	})

	t.Run("ConfigUpdated should record config errors on the cfgm object", func(t *testing.T) {
		beforeEach()
		assert := assert.New(t)

		nc := config.NewDefaultConfig()
		mc := &pb.MainConfig{}
		e := fmt.Errorf("test error")
		configMapParser.On("Parse", &cfgm).Return(nc, errors.WrapInObjectContext(parser.ValidationError([]error{e}), &cfgm))
		recorder.On("Event", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		renderer.On("RenderMainConfig", mock.Anything).Return(mc, nil)
		mainConfigStorage.On("Put", mc).Return(nil)

		err := c.ConfigUpdated(&cfgm)
		assert.NoError(err)
		configMapParser.AssertCalled(t, "Parse", &cfgm)
		renderer.AssertCalled(t, "RenderMainConfig", nc)
		mainConfigStorage.AssertCalled(t, "Put", mc)
		recorder.AssertCalled(t, "Event", &cfgm, api_v1.EventTypeWarning, "Config Error", mock.Anything)
	})
}
