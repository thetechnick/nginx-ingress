package agent

import (
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
)

// StatusServer is used to query the agent status from the outside
type StatusServer interface {
	Run() error
}

// NewStatusServer creates a new StatusServer
func NewStatusServer(readyCh <-chan interface{}) StatusServer {
	s := &statusServer{
		log: log.WithField("module", "StatusServer"),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/ready", s.readyHandler)

	server := &http.Server{
		Addr:    "0.0.0.0:9000",
		Handler: mux,
	}
	s.server = server

	go func() {
		<-readyCh
		s.log.Info("Status changed to ready")
		s.ready = true
	}()

	return s
}

type statusServer struct {
	ready  bool
	server *http.Server
	log    *log.Entry
}

func (s *statusServer) Run() error {
	err := s.server.ListenAndServe()
	if err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *statusServer) readyHandler(w http.ResponseWriter, r *http.Request) {
	if s.ready {
		fmt.Fprint(w, "ready")
		return
	}
	http.Error(w, "not ready", http.StatusServiceUnavailable)
}
