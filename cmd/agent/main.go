package main

import (
	"context"
	"flag"
	"strings"
	"time"

	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/agent"
	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/shell"
	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/storage/local"

	"github.com/coreos/etcd/clientv3"
	log "github.com/sirupsen/logrus"
)

func main() {
	endpointsString := flag.String("etcd-endpoints", "localhost:2379",
		`ETCD endpoints`)
	flag.Parse()

	endpoints := strings.Split(*endpointsString, ",")

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.WithError(err).Fatal("Error creating client")
	}
	defer cli.Close()

	n := agent.NewNginx(shell.NewLogExecutor())
	scs := local.NewServerConfigStorage(n)
	mcs := local.NewMainConfigStorage(n)
	a := agent.NewAgent(cli, scs, mcs)
	go n.Run()
	a.Run(context.Background())
}
