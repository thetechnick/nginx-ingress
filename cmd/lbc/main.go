package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/golang/glog"
	log "github.com/sirupsen/logrus"
	"github.com/thetechnick/nginx-ingress/pkg/agent"
	"github.com/thetechnick/nginx-ingress/pkg/controller"
	"github.com/thetechnick/nginx-ingress/pkg/storage/etcd"
	"github.com/thetechnick/nginx-ingress/pkg/storage/local"
	"github.com/thetechnick/nginx-ingress/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var (
	endpointsString = flag.String("etcd-endpoints", "localhost:2379",
		`ETCD endpoints`)

	serverMode = flag.Bool("server-mode", false, `If true, writes configs into etcd where agents can listen for config changes. If false, startes a local nginx instance to configure`)

	healthStatus = flag.Bool("health-status", false,
		`If present, the default server listening on port 80 with the health check
		location "/nginx-health" gets added to the main nginx configuration.`)

	proxyURL = flag.String("proxy", "",
		`If specified, the controller assumes a kubctl proxy server is running on the
		given url and creates a proxy client. Regenerated NGINX configuration files
    are not written to the disk, instead they are printed to stdout. Also NGINX
    is not getting invoked. This flag is for testing.`)

	watchNamespace = flag.String("watch-namespace", api.NamespaceAll,
		`Namespace to watch for Ingress/Services/Endpoints. By default the controller
		watches acrosss all namespaces`)

	logLevel = flag.String("log-level", "info",
		`Log level can be one of "debug", "info", "warning", "error", "fatal"`)

	nginxConfigMaps = flag.String("nginx-configmaps", "",
		`Specifies a configmaps resource that can be used to customize NGINX
		configuration. The value must follow the following format: <namespace>/<name>`)

	printVersion = flag.Bool("version", false, "Print version and exit")
)

func main() {
	flag.Parse()
	if *printVersion {
		fmt.Printf("NGINX Ingress controller version: %s\n", version.Version)
		return
	}

	flag.Lookup("logtostderr").Value.Set("true")
	log.SetFormatter(&log.JSONFormatter{})
	var err error
	level, err := log.ParseLevel(*logLevel)
	if err != nil {
		log.WithError(err).Fatal("unable to set log level")
	}
	log.SetLevel(level)
	log.SetOutput(os.Stderr)

	log.Infof("Starting NGINX loadbalancer controller Version %v\n", version.Version)

	var config *rest.Config
	if *proxyURL != "" {
		config, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{},
			&clientcmd.ConfigOverrides{
				ClusterInfo: clientcmdapi.Cluster{
					Server: *proxyURL,
				},
			}).ClientConfig()
		if err != nil {
			log.Fatalf("error creating client configuration: %v", err)
		}
	} else {
		if config, err = rest.InClusterConfig(); err != nil {
			log.Fatalf("error creating client configuration: %v", err)
		}
	}

	glog.V(0) // disable glog of kube-client
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v.", err)
	}

	var lbc *controller.LoadBalancerController
	if *serverMode {
		log.Info("NGINX loadbalancer controller running in server mode")
		endpoints := strings.Split(*endpointsString, ",")
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   endpoints,
			DialTimeout: 120 * time.Second,
		})
		if err != nil {
			log.WithError(err).Fatal("Error creating client")
		}
		defer cli.Close()

		mcs := etcd.NewMainConfigStorage(cli)
		scs := etcd.NewServerConfigStorage(cli)

		lbc, _ = controller.NewLoadBalancerController(
			kubeClient,
			30*time.Second,
			*watchNamespace,
			*nginxConfigMaps,
			mcs,
			scs,
		)
		lbc.Run()

		return
	}

	log.Info("NGINX loadbalancer controller running in local mode")

	n := agent.NewNginx(nil)
	mcs := local.NewMainConfigStorage(n)
	scs := local.NewServerConfigStorage(n)

	lbc, _ = controller.NewLoadBalancerController(
		kubeClient,
		30*time.Second,
		*watchNamespace,
		*nginxConfigMaps,
		mcs,
		scs,
	)

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGTERM, syscall.SIGINT)

	nginxStopped := make(chan error, 1)
	go lbc.Run()
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
		lbc.Stop()
		n.Stop()
		<-nginxStopped
	}
}
