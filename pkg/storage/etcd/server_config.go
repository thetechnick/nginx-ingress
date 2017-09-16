package etcd

import (
	"context"

	"github.com/coreos/etcd/clientv3"
	"github.com/gogo/protobuf/proto"
	log "github.com/sirupsen/logrus"
	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/storage"
	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/storage/pb"
)

const (
	// ServerConfigKeyPrefix is used to prefix server config keys
	ServerConfigKeyPrefix = "lbc/server/"
)

// NewServerConfigStorage returns a ServerConfigStorage working on etcd v3
func NewServerConfigStorage(client *clientv3.Client) storage.ServerConfigStorage {
	return &etcdServerStorage{
		client: client,
		log:    log.WithField("module", "EtcdServerConfigStorage"),
	}
}

type etcdServerStorage struct {
	client *clientv3.Client
	log    *log.Entry
}

func getKeyName(cfg *pb.ServerConfig) string {
	name := cfg.Name
	if name == "" {
		name = "default"
	}
	return ServerConfigKeyPrefix + name
}

func (s *etcdServerStorage) Put(cfg *pb.ServerConfig) error {
	existingCfg, err := s.Get(cfg.Name)
	if err != nil {
		return err
	}
	if existingCfg != nil && proto.Equal(existingCfg, cfg) {
		s.log.WithField("name", cfg.Name).Info("resource is already up to date, skipped")
		return nil
	}

	b, err := proto.Marshal(cfg)
	if err != nil {
		return err
	}

	_, err = s.client.Put(context.Background(), getKeyName(cfg), string(b))
	if err != nil {
		return err
	}

	return nil
}

func (s *etcdServerStorage) Delete(cfg *pb.ServerConfig) error {
	_, err := s.client.Delete(context.Background(), getKeyName(cfg))
	if err != nil {
		return err
	}

	return nil
}

func (s *etcdServerStorage) List() ([]*pb.ServerConfig, error) {
	resp, err := s.client.Get(context.Background(), ServerConfigKeyPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	cfgs := []*pb.ServerConfig{}
	for _, kv := range resp.Kvs {
		sc := &pb.ServerConfig{}
		if err := proto.Unmarshal(kv.Value, sc); err != nil {
			return nil, err
		}
		cfgs = append(cfgs, sc)
	}

	return cfgs, nil
}

func (s *etcdServerStorage) ByIngressKey(ingressKey string) ([]*pb.ServerConfig, error) {
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

func (s *etcdServerStorage) Get(name string) (*pb.ServerConfig, error) {
	resp, err := s.client.Get(context.Background(), ServerConfigKeyPrefix+name)
	if err != nil {
		return nil, err
	}

	for _, kv := range resp.Kvs {
		sc := &pb.ServerConfig{}
		if err := proto.Unmarshal(kv.Value, sc); err != nil {
			return nil, err
		}
		return sc, nil
	}

	return nil, nil
}
