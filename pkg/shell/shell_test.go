package shell

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExec(t *testing.T) {
	t.Run("runs a command", func(t *testing.T) {
		e := NewShellExecutor()
		assert := assert.New(t)
		assert.NoError(e.Exec("true"))
	})

	t.Run("returns a error containing the process output", func(t *testing.T) {
		e := NewShellExecutor()
		assert := assert.New(t)
		err := e.Exec("echo test && false")
		if assert.Error(err) {
			e := err.(ExecError)
			assert.Equal("exit status 1", e.Error())
			assert.Equal("test\n", string(e.Output))
		}
	})
}
