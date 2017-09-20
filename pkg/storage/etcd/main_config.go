package etcd

import (
	"context"

	"github.com/coreos/etcd/clientv3"
	"github.com/gogo/protobuf/proto"
	log "github.com/sirupsen/logrus"
	"github.com/thetechnick/nginx-ingress/pkg/storage"
	"github.com/thetechnick/nginx-ingress/pkg/storage/pb"
)

const (
	// MainConfigKey is used as key of the main config
	MainConfigKey = "lbc/main-config"
)

// NewMainConfigStorage stores the main config in etcd
func NewMainConfigStorage(client *clientv3.Client) storage.MainConfigStorage {
	return &etcdMainConfigStorage{
		client: client,
		log:    log.WithField("module", "EtcdMainConfigStorage"),
	}
}

type etcdMainConfigStorage struct {
	client *clientv3.Client
	log    *log.Entry
}

func (s *etcdMainConfigStorage) Get() (*pb.MainConfig, error) {
	resp, err := s.client.Get(context.Background(), MainConfigKey)
	if err != nil {
		return nil, err
	}

	for _, kv := range resp.Kvs {
		mc := &pb.MainConfig{}
		if err := proto.Unmarshal(kv.Value, mc); err != nil {
			return nil, err
		}
		return mc, nil
	}

	return nil, nil
}

func (s *etcdMainConfigStorage) Put(cfg *pb.MainConfig) error {
	b, err := proto.Marshal(cfg)
	if err != nil {
		return err
	}

	_, err = s.client.Put(context.Background(), MainConfigKey, string(b))
	if err != nil {
		return err
	}

	return nil
}
