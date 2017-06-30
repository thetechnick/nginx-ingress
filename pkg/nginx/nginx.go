package nginx

import (
	log "github.com/sirupsen/logrus"

	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/shell"
)

// Nginx controls the nginx process
type Nginx interface {
	Start() error
	Stop() error
	Reload() error
	TestConfig() error
}

// NewNginx creates a new nginx instance
func NewNginx(e shell.Executor) Nginx {
	return &nginx{e}
}

type nginx struct {
	executor shell.Executor
}

func (n *nginx) Start() error {
	log.Debug("starting nginx")
	return n.executor.Exec("nginx")
}

func (n *nginx) Stop() error {
	log.Debug("stopping nginx")
	return n.executor.Exec("nginx -s quit")
}

func (n *nginx) Reload() error {
	log.Debug("reloading nginx")
	if err := n.TestConfig(); err != nil {
		return err
	}
	if err := n.executor.Exec("nginx -s reload"); err != nil {
		return err
	}
	return nil
}

func (n *nginx) TestConfig() error {
	log.Debug("test nginx config")
	return n.executor.Exec("nginx -t")
}
