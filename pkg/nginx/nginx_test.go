package nginx

import (
	"testing"

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

func TestNginx(t *testing.T) {
	t.Run("Start should start nginx", func(t *testing.T) {
		assert := assert.New(t)
		e := &ExecutorMock{}
		e.On("Exec", "nginx").Return(nil)
		n := NewNginx(e)

		assert.NoError(n.Start())
	})

	t.Run("Stop should stop nginx", func(t *testing.T) {
		assert := assert.New(t)
		e := &ExecutorMock{}
		e.On("Exec", "nginx -s quit").Return(nil)
		n := NewNginx(e)

		assert.NoError(n.Stop())
	})

	t.Run("Reload should reload nginx", func(t *testing.T) {
		assert := assert.New(t)
		e := &ExecutorMock{}
		e.On("Exec", "nginx -t").Return(nil)
		e.On("Exec", "nginx -s reload").Return(nil)
		n := NewNginx(e)

		assert.NoError(n.Reload())
	})
}
