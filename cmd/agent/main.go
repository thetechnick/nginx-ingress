package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/coreos/etcd/clientv3"
	log "github.com/sirupsen/logrus"
	"github.com/thetechnick/nginx-ingress/pkg/agent"
	"github.com/thetechnick/nginx-ingress/pkg/storage/local"
	"github.com/thetechnick/nginx-ingress/pkg/version"
)

var (
	endpointsString = flag.String("etcd-endpoints", "localhost:2379",
		`ETCD endpoints`)

	printVersion = flag.Bool("version", false, "Print version and exit")
	logLevel     = flag.String("log-level", "info",
		`Log level can be one of "debug", "info", "warning", "error", "fatal"`)
)

func main() {
	flag.Parse()
	if *printVersion {
		fmt.Printf("NGINX Ingress controller version: %s\n", version.Version)
		return
	}

	level, err := log.ParseLevel(*logLevel)
	if err != nil {
		log.WithError(err).Fatal("unable to set log level")
	}
	log.SetLevel(level)
	log.SetOutput(os.Stderr)

	endpoints := strings.Split(*endpointsString, ",")
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.WithError(err).Fatal("Error creating client")
	}
	defer cli.Close()

	n := agent.NewNginx(nil)
	scs := local.NewServerConfigStorage(n)
	mcs := local.NewMainConfigStorage(n)

	readyCh := make(chan interface{}, 1)
	a := agent.NewAgent(cli, scs, mcs, readyCh)
	statusServer := agent.NewStatusServer(readyCh)

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGTERM, syscall.SIGINT)

	nginxStopped := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Infof("Starting NGINX Ingress agent Version %v", version.Version)
	go a.Run(ctx)
	go statusServer.Run()
	go func() {
		nginxStopped <- n.Run()
	}()

	select {
	case err := <-nginxStopped:
		if err != nil {
			log.WithError(err).Fatal("NGINX process exited with error")
		}
		log.Fatal("NGINX process exited unexpectedly")

	case <-signalCh:
		log.Info("Received SIGTERM, stopping gracefully")
		cancel()
		n.Stop()
		<-nginxStopped
	}
}
