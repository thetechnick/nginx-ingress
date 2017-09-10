package agent

import (
	log "github.com/sirupsen/logrus"
	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/shell"
)

// Nginx controls a nginx process
type Nginx interface {
	Run() error
	Stop() error
	Reload() error
	TestConfig() error
}

type nginx struct {
	log      *log.Entry
	executor shell.Executor
}

// NewNginx creates a new nginx instance
func NewNginx(e shell.Executor) Nginx {
	if e == nil {
		e = shell.NewShellExecutor()
	}
	return &nginx{
		log:      log.WithField("module", "nginx"),
		executor: e,
	}
}

func (n *nginx) Run() error {
	n.log.Debug("starting nginx")
	err := n.executor.Exec("nginx -g 'daemon off;'")
	n.log.WithError(err).Debug("nginx stopped")
	return err
}

func (n *nginx) Stop() error {
	n.log.Debug("stopping nginx")
	return n.executor.Exec("nginx -s quit")
}

func (n *nginx) Reload() error {
	n.log.Debug("reloading nginx")
	if err := n.TestConfig(); err != nil {
		return err
	}
	if err := n.executor.Exec("nginx -s reload"); err != nil {
		return err
	}
	return nil
}

func (n *nginx) TestConfig() error {
	n.log.Debug("test nginx config")
	return n.executor.Exec("nginx -t")
}
