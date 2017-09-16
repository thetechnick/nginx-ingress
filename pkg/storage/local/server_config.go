package local

import (
	"path"
	"sync"

	"github.com/gogo/protobuf/proto"
	log "github.com/sirupsen/logrus"
	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/agent"
	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/storage"
	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/storage/pb"
)

// NewServerConfigStorage stores the config in the file system configures a local nginx instance
func NewServerConfigStorage(n agent.Nginx) storage.ServerConfigStorage {
	return &localServerStorage{
		nginx:             n,
		log:               log.WithField("module", "LocalServerConfigStorage"),
		createTransaction: CreateTransaction,
		store:             map[string]*pb.ServerConfig{},
	}
}

type localServerStorage struct {
	nginx             agent.Nginx
	log               *log.Entry
	createTransaction TransactionFactory
	mutex             sync.Mutex
	store             map[string]*pb.ServerConfig
}

func (s *localServerStorage) Put(cfg *pb.ServerConfig) error {
	existingCfg, err := s.Get(cfg.Name)
	if err != nil {
		return err
	}
	if existingCfg != nil && proto.Equal(existingCfg, cfg) {
		s.log.WithField("name", cfg.Name).Info("resource is already up to date, skipped")
		return nil
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.log.WithField("key", cfg.Name).WithField("meta", cfg.Meta).Debug("Put")
	t := s.createTransaction()
	defer t.Apply()

	err = t.Update(s.getServerConfigFilename(cfg), string(cfg.Config))
	if err != nil {
		t.Rollback()
		return err
	}

	if cfg.Tls != nil {
		filename := path.Join(storage.MainConfigDir, cfg.Tls.Name)
		err = t.Update(filename, string(cfg.Tls.Content))
		if err != nil {
			t.Rollback()
			return err
		}
	}

	err = s.nginx.Reload()
	if err != nil {
		t.Rollback()
		return err
	}

	// update store
	s.store[cfg.Name] = cfg

	return nil
}

func (s *localServerStorage) Delete(cfg *pb.ServerConfig) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.log.WithField("key", cfg.Name).WithField("meta", cfg.Meta).Debug("Delete")

	t := s.createTransaction()
	defer t.Apply()

	err := t.Delete(s.getServerConfigFilename(cfg))
	if err != nil {
		t.Rollback()
		return err
	}

	if cfg.Tls != nil {
		filename := path.Join(storage.MainConfigDir, cfg.Tls.Name)
		err = t.Delete(filename)
		if err != nil {
			t.Rollback()
			return err
		}
	}

	err = s.nginx.Reload()
	if err != nil {
		t.Rollback()
		return err
	}

	// update store
	delete(s.store, cfg.Name)

	return nil
}

func (s *localServerStorage) Get(name string) (*pb.ServerConfig, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.store[name], nil
}

func (s *localServerStorage) List() ([]*pb.ServerConfig, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	cfgs := []*pb.ServerConfig{}
	for _, s := range s.store {
		cfgs = append(cfgs, s)
	}
	return cfgs, nil
}

func (s *localServerStorage) ByIngressKey(ingressKey string) ([]*pb.ServerConfig, error) {
	servers, err := s.List()
	if err != nil {
		return nil, err
	}

	matching := []*pb.ServerConfig{}
	for _, serverConfig := range servers {
		if _, ok := serverConfig.Meta[ingressKey]; ok {
			matching = append(matching, serverConfig)
		}
	}
	return matching, nil
}

func (s *localServerStorage) getServerConfigFilename(cfg *pb.ServerConfig) string {
	name := cfg.Name
	if cfg.Name == "" {
		name = "default"
	}
	return path.Join(storage.ServerConfigDir, name+".conf")
}
