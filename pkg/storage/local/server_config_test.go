package local

import (
	"testing"

	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/storage/pb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type NginxMock struct {
	mock.Mock
}

func (e *NginxMock) Run() error {
	args := e.Called()
	return args.Error(0)
}
func (e *NginxMock) Stop() error {
	args := e.Called()
	return args.Error(0)
}
func (e *NginxMock) Reload() error {
	args := e.Called()
	return args.Error(0)
}
func (e *NginxMock) TestConfig() error {
	args := e.Called()
	return args.Error(0)
}

type TransactionMock struct {
	mock.Mock
}

func (m *TransactionMock) Delete(filename string) error {
	args := m.Called(filename)
	return args.Error(0)
}

func (m *TransactionMock) Update(filename, content string) error {
	args := m.Called(filename, content)
	return args.Error(0)
}

func (m *TransactionMock) Rollback() {
	m.Called()
}

func (m *TransactionMock) Apply() {
	m.Called()
}

func TestNewServerConfigStorage(t *testing.T) {
	assert := assert.New(t)
	assert.NotNil(NewServerConfigStorage(nil))
}

func TestServerConfigStorage(t *testing.T) {
	exampleServerConfig := &pb.ServerConfig{
		Name: "test",
		Meta: map[string]string{
			"test": "test",
		},
		Tls: &pb.TLSCertificate{
			Name:    "ssl/test.pem",
			Content: []byte("\n"),
		},
	}

	var cm *localServerStorage
	var nginxMock *NginxMock
	var transactionMock *TransactionMock

	beforeEach := func() {
		cm = NewServerConfigStorage(nil).(*localServerStorage)

		nginxMock = &NginxMock{}
		transactionMock = &TransactionMock{}
		cm.createTransaction = func() Transaction {
			return transactionMock
		}
		cm.nginx = nginxMock
	}

	t.Run("Put", func(t *testing.T) {
		beforeEach()
		assert := assert.New(t)

		nginxMock.On("Reload").Return(nil)
		transactionMock.On("Update", "/etc/nginx/conf.d/test.conf", "").Return(nil)
		transactionMock.On("Update", "/etc/nginx/ssl/test.pem", "\n").Return(nil)
		transactionMock.On("Apply")

		err := cm.Put(exampleServerConfig)
		err2 := cm.Put(exampleServerConfig)

		if assert.NoError(err) && assert.NoError(err2) {
			nginxMock.AssertCalled(t, "Reload")
			nginxMock.AssertNumberOfCalls(t, "Reload", 1)

			transactionMock.AssertCalled(t, "Update", "/etc/nginx/conf.d/test.conf", "")
			transactionMock.AssertCalled(t, "Update", "/etc/nginx/ssl/test.pem", "\n")
			transactionMock.AssertNumberOfCalls(t, "Update", 2)

			transactionMock.AssertCalled(t, "Apply")
			transactionMock.AssertNumberOfCalls(t, "Apply", 1)
		}
	})

	t.Run("Get", func(t *testing.T) {
		assert := assert.New(t)

		cfg, err := cm.Get("test")
		if assert.NoError(err) && assert.NotNil(cfg) {
			assert.Equal(exampleServerConfig, cfg)
		}
	})

	t.Run("List", func(t *testing.T) {
		assert := assert.New(t)
		cfgs, err := cm.List()
		if assert.NoError(err) && assert.Len(cfgs, 1) {
			assert.Equal(exampleServerConfig, cfgs[0])
		}
	})

	t.Run("Delete", func(t *testing.T) {
		assert := assert.New(t)

		nginxMock.On("Reload").Return(nil)
		transactionMock.On("Delete", "/etc/nginx/conf.d/test.conf").Return(nil)
		transactionMock.On("Delete", "/etc/nginx/ssl/test.pem").Return(nil)
		transactionMock.On("Apply")

		err := cm.Delete(&pb.ServerConfig{
			Name: "test",
			Tls: &pb.TLSCertificate{
				Name:    "ssl/test.pem",
				Content: []byte("\n"),
			},
		})
		if assert.NoError(err) {
			nginxMock.AssertCalled(t, "Reload")
			transactionMock.AssertCalled(t, "Delete", "/etc/nginx/conf.d/test.conf")
			transactionMock.AssertCalled(t, "Delete", "/etc/nginx/ssl/test.pem")
			transactionMock.AssertCalled(t, "Apply")
		}
	})
}
