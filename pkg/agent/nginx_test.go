package agent

import (
	"errors"
	"testing"

	"github.com/thetechnick/nginx-ingress/pkg/shell"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type ExecutorMock struct {
	mock.Mock
}

func (e *ExecutorMock) Exec(cmd string) error {
	args := e.Called(cmd)
	return args.Error(0)
}

func newNginx(e shell.Executor) Nginx {
	return &nginx{
		log:      log.WithField("t", "t"),
		executor: e,
	}
}

func TestNewNginx(t *testing.T) {
	assert := assert.New(t)
	n := NewNginx(nil)
	assert.NotNil(n)
}

func TestNginx(t *testing.T) {
	t.Run("Start should start nginx", func(t *testing.T) {
		assert := assert.New(t)
		e := &ExecutorMock{}
		e.On("Exec", "nginx -g 'daemon off;'").Return(nil)
		n := newNginx(e)

		assert.NoError(n.Run())
	})

	t.Run("Stop should stop nginx", func(t *testing.T) {
		assert := assert.New(t)
		e := &ExecutorMock{}
		e.On("Exec", "nginx -s quit").Return(nil)
		n := newNginx(e)

		assert.NoError(n.Stop())
		e.AssertCalled(t, "Exec", "nginx -s quit")
	})

	t.Run("Reload should reload nginx", func(t *testing.T) {
		assert := assert.New(t)
		e := &ExecutorMock{}
		e.On("Exec", "nginx -t").Return(nil)
		e.On("Exec", "nginx -s reload").Return(nil)
		n := newNginx(e)

		assert.NoError(n.Reload())
		e.AssertCalled(t, "Exec", "nginx -s reload")
	})

	t.Run("Reload should test config before reloading nginx", func(t *testing.T) {
		assert := assert.New(t)
		err := errors.New("error")

		e := &ExecutorMock{}
		e.On("Exec", "nginx -t").Return(err)
		e.On("Exec", "nginx -s reload").Return(nil)
		n := newNginx(e)

		assert.EqualError(n.Reload(), err.Error())
		e.AssertCalled(t, "Exec", "nginx -t")
		e.AssertNotCalled(t, "Exec", "nginx -s reload")
	})
}
