package shell

import (
	"os/exec"

	log "github.com/sirupsen/logrus"
)

// ExecError is returned by Executors
type ExecError struct {
	Err    error
	Output []byte
}

func (e ExecError) Error() string {
	return e.Err.Error()
}

// Executor executes shell-style commands
type Executor interface {
	Exec(cmd string) error
}

// NewShellExecutor executes the given command in a shell
func NewShellExecutor() Executor {
	return &shellExecutor{}
}

type shellExecutor struct{}

func (e *shellExecutor) Exec(shellCommand string) error {
	cmd := exec.Command("sh", "-c", shellCommand)
	log.WithField("cmd", shellCommand).Debug("executing shell command")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return ExecError{
			Err:    err,
			Output: output,
		}
	}
	return nil
}
