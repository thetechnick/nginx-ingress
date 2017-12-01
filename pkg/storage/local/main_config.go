package local

import (
	"path"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/thetechnick/nginx-ingress/pkg/agent"
	"github.com/thetechnick/nginx-ingress/pkg/storage"
	"github.com/thetechnick/nginx-ingress/pkg/storage/pb"
)

const (
	nginxFilename = "nginx.conf"
)

// NewMainConfigStorage stores the config in the file system configures a local nginx instance
func NewMainConfigStorage(n agent.Nginx) storage.MainConfigStorage {
	return &localMainConfigStorage{
		nginx:             n,
		log:               log.WithField("module", "LocalMainConfigStorage"),
		createTransaction: CreateTransaction,
	}
}

type localMainConfigStorage struct {
	nginx             agent.Nginx
	log               *log.Entry
	createTransaction TransactionFactory
	mutex             sync.Mutex
	store             *pb.MainConfig
}

func (s *localMainConfigStorage) Get() (*pb.MainConfig, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.store, nil
}

func (s *localMainConfigStorage) Put(cfg *pb.MainConfig) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	filename := path.Join(storage.MainConfigDir, nginxFilename)

	t := s.createTransaction()
	defer t.Apply()
	err := t.Update(filename, string(cfg.Config))
	if err != nil {
		t.Rollback()
		return err
	}

	if cfg.Dhparam != nil {
		err = t.Update(storage.DHParamFile, string(cfg.Dhparam))
		if err != nil {
			t.Rollback()
			return err
		}
	}

	for _, file := range cfg.Files {
		err = t.Update(file.Name, string(file.Content))
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

	s.store = cfg

	return nil
}
