package agent

import (
	"context"

	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/storage"
	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/storage/pb"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"
)

const (
	serverKeyPrefix = "lbc/server/"
	mainConfigKey   = "lbc/main-config"
)

// Agent watches etcd to trigger config updates
type Agent struct {
	serverConfigStorage storage.ServerConfigStorage
	mainConfigStorage   storage.MainConfigStorage
	client              *clientv3.Client
	log                 *log.Entry
}

// NewAgent creates a new Agent instance
func NewAgent(client *clientv3.Client, scs storage.ServerConfigStorage, mcs storage.MainConfigStorage) *Agent {
	return &Agent{scs, mcs, client, log.WithField("module", "agent")}
}

func (a *Agent) runServerWatcher() {
	a.log.Info("Starting Server watcher")

	wch := a.client.Watch(context.Background(), serverKeyPrefix, clientv3.WithPrefix(), clientv3.WithPrevKV())
	for wresp := range wch {
		for _, e := range wresp.Events {
			a.handleServerEvent(e)
		}
	}
}

func (a *Agent) runMainConfigWatcher() {
	a.log.Info("Starting MainConfig watcher")

	wch := a.client.Watch(context.Background(), mainConfigKey, clientv3.WithPrevKV())
	for wresp := range wch {
		for _, e := range wresp.Events {
			a.handleMainConfigEvent(e)
		}
	}
}

func (a *Agent) serverFromKey(kv *mvccpb.KeyValue) *pb.ServerConfig {
	s := &pb.ServerConfig{}
	if err := proto.Unmarshal(kv.Value, s); err != nil {
		a.log.
			WithField("key", string(kv.Key)).
			WithError(err).
			Error("Unmarshal error, skipping")
		return nil
	}
	return s
}

func (a *Agent) mainConfigFromKey(kv *mvccpb.KeyValue) *pb.MainConfig {
	c := &pb.MainConfig{}
	if err := proto.Unmarshal(kv.Value, c); err != nil {
		a.log.
			WithField("key", string(kv.Key)).
			WithError(err).
			Error("Unmarshal error, skipping")
		return nil
	}
	return c
}

func (a *Agent) updateMainConfigFromKey(kv *mvccpb.KeyValue) {
	c := a.mainConfigFromKey(kv)
	if c == nil {
		return
	}

	if err := a.mainConfigStorage.Put(c); err != nil {
		a.log.
			WithField("key", string(kv.Key)).
			WithError(err).
			Error("Error updating MainConfig")
		return
	}
}

func (a *Agent) deleteServerFromKey(kv *mvccpb.KeyValue) {
	s := a.serverFromKey(kv)
	if s == nil {
		return
	}

	if err := a.serverConfigStorage.Delete(s); err != nil {
		a.log.
			WithField("key", string(kv.Key)).
			WithError(err).
			Error("Error deleting Server")
	}
}

func (a *Agent) updateServerFromKey(kv *mvccpb.KeyValue) {
	s := a.serverFromKey(kv)
	if s == nil {
		return
	}

	if err := a.serverConfigStorage.Put(s); err != nil {
		a.log.
			WithField("key", string(kv.Key)).
			WithError(err).
			Error("Error updating Server")
	}
}

func (a *Agent) handleMainConfigEvent(event *clientv3.Event) {
	a.log.
		WithField("key", string(event.Kv.Key)).
		Debug("MainConfig key changed")

	if event.Type == clientv3.EventTypeDelete {
		a.log.
			WithField("key", string(event.Kv.Key)).
			Error("MainConfig key deleted, still using old config")
		return
	}

	a.updateMainConfigFromKey(event.Kv)
}

func (a *Agent) handleServerEvent(event *clientv3.Event) {
	a.log.
		WithField("key", string(event.Kv.Key)).
		Debug("Server key changed")

	if event.Type == clientv3.EventTypeDelete {
		a.deleteServerFromKey(event.PrevKv)
		return
	}

	a.updateServerFromKey(event.Kv)
}

func (a *Agent) syncExistingServers() {
	a.log.
		Info("Syncing existing Servers")
	resp, err := a.client.Get(context.Background(), serverKeyPrefix, clientv3.WithPrefix())
	if err != nil {
		a.log.
			WithError(err).
			Debug("Error syncing servers")
		return
	}

	for _, kv := range resp.Kvs {
		a.updateServerFromKey(kv)
	}
}

func (a *Agent) syncMainConfig() {
	a.log.
		Info("Syncing existing MainConfig")
	resp, err := a.client.Get(context.Background(), mainConfigKey)
	if err != nil {
		a.log.
			WithError(err).
			Debug("Error syncing servers")
		return
	}

	for _, kv := range resp.Kvs {
		a.updateMainConfigFromKey(kv)
	}
}

// Run starts the agent
func (a *Agent) Run(ctx context.Context) {
	// start watchers
	go a.runMainConfigWatcher()
	go a.runServerWatcher()

	// sync existing config
	a.syncMainConfig()
	a.syncExistingServers()

	// 3. set ready state

	// block
	<-ctx.Done()
}
