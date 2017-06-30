package nginx

import (
	"fmt"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type nginxMock struct {
	mock.Mock
}

func (m *nginxMock) Reload() error {
	args := m.Called()
	return args.Error(0)
}

func (m *nginxMock) Start() error {
	args := m.Called()
	return args.Error(0)
}

func (m *nginxMock) Stop() error {
	args := m.Called()
	return args.Error(0)
}

func (m *nginxMock) TestConfig() error {
	args := m.Called()
	return args.Error(0)
}

func TestControllerStart(t *testing.T) {
	t.Run("should start nginx", func(t *testing.T) {
		nginx := &nginxMock{}
		c := Controller{
			nginxConfdPath: path.Join("/tmp", "conf.d"),
			nginxCertsPath: path.Join("/tmp", "ssl"),
			local:          false,
			healthStatus:   false,
			nginx:          nginx,
		}
		nginx.On("Start").Return(nil)
		c.Start()
	})
}

func TestControllerReload(t *testing.T) {
	t.Run("should reload nginx after testing", func(t *testing.T) {
		assert := assert.New(t)
		nginx := &nginxMock{}
		c := Controller{
			nginxConfdPath: path.Join("/tmp", "conf.d"),
			nginxCertsPath: path.Join("/tmp", "ssl"),
			local:          false,
			healthStatus:   false,
			nginx:          nginx,
		}
		nginx.On("TestConfig").Return(nil)
		nginx.On("Reload").Return(nil)

		assert.NoError(c.Reload())
	})

	t.Run("should not reload if the config is invalid", func(t *testing.T) {
		assert := assert.New(t)
		nginx := &nginxMock{}
		c := Controller{
			nginxConfdPath: path.Join("/tmp", "conf.d"),
			nginxCertsPath: path.Join("/tmp", "ssl"),
			local:          false,
			healthStatus:   false,
			nginx:          nginx,
		}
		nginx.On("TestConfig").Return(fmt.Errorf("error"))

		err := c.Reload()
		if assert.Error(err) {
			assert.Contains(err.Error(), "Invalid nginx configuration detected")
		}
	})
}
