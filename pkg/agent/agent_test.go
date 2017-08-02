package agent

import (
	"context"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/mock"
	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/storage/pb"
	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/test"
)

var (
	testServer = &pb.ServerConfig{
		Config: []byte("config"),
	}

	testMainConfig = &pb.MainConfig{
		Config: []byte("config"),
	}
)

func TestAgent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping agent integration test")
	}

	scsMock := &test.ServerConfigStorageMock{}
	mcsMock := &test.MainConfigStorageMock{}

	testCLI, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Error(err)
	}
	defer testCLI.Close()

	// delete all keys
	testCLI.Delete(context.Background(), "", clientv3.WithPrefix())

	t.Run("Watch MainConfig Update", func(t *testing.T) {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   []string{"localhost:2379"},
			DialTimeout: 5 * time.Second,
		})
		if err != nil {
			t.Error(err)
		}
		defer cli.Close()

		a := NewAgent(cli, scsMock, mcsMock)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go a.Run(ctx)

		updateCh := make(chan interface{}, 1)
		mcsMock.On("Put", testMainConfig).Run(func(arg1 mock.Arguments) {
			updateCh <- nil
		}).Return(nil)

		mcb, err := proto.Marshal(testMainConfig)
		if err != nil {
			t.Error(err)
		}
		_, err = testCLI.Put(context.Background(), mainConfigKey, string(mcb))
		if err != nil {
			t.Error(err)
		}

		<-updateCh
		mcsMock.AssertCalled(t, "Put", testMainConfig)
	})

	t.Run("Watch Servers Update", func(t *testing.T) {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   []string{"localhost:2379"},
			DialTimeout: 5 * time.Second,
		})
		if err != nil {
			t.Error(err)
		}
		defer cli.Close()

		a := NewAgent(cli, scsMock, mcsMock)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go a.Run(ctx)

		updateServerCh := make(chan interface{}, 1)
		scsMock.On("Put", testServer).Run(func(arg1 mock.Arguments) {
			updateServerCh <- nil
		}).Return(nil)

		sb, err := proto.Marshal(testServer)
		if err != nil {
			t.Error(err)
		}
		_, err = testCLI.Put(context.Background(), serverKeyPrefix+"test", string(sb))
		if err != nil {
			t.Error(err)
		}

		<-updateServerCh
		scsMock.AssertCalled(t, "Put", testServer)
	})

	t.Run("Watch Servers Delete", func(t *testing.T) {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   []string{"localhost:2379"},
			DialTimeout: 5 * time.Second,
		})
		if err != nil {
			t.Error(err)
		}
		defer cli.Close()

		a := NewAgent(cli, scsMock, mcsMock)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go a.Run(ctx)

		deleteServerCh := make(chan interface{}, 1)
		scsMock.On("Delete", testServer).Run(func(arg1 mock.Arguments) {
			deleteServerCh <- nil
		}).Return(nil)
		_, err = testCLI.Delete(context.Background(), "lbc/server/test")
		if err != nil {
			t.Error(err)
		}
		<-deleteServerCh
		scsMock.AssertCalled(t, "Delete", testServer)
	})
}
