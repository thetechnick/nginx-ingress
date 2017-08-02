package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang/glog"
	log "github.com/sirupsen/logrus"
	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/controller"
	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var (
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

	logLevel = flag.String("log-level", "warning",
		`Log level can be one of "debug", "info", "warning", "error", "fatal"`)

	nginxConfigMaps = flag.String("nginx-configmaps", "",
		`Specifies a configmaps resource that can be used to customize NGINX
		configuration. The value must follow the following format: <namespace>/<name>`)

	printVersion = flag.Bool("version", false, "Print version and exit")
)

func main() {
	flag.Parse()
	flag.Lookup("logtostderr").Value.Set("true")
	log.SetFormatter(&log.JSONFormatter{})
	var err error
	level, err := log.ParseLevel(*logLevel)
	if err != nil {
		log.WithError(err).Fatal("unable to set log level")
	}
	log.SetLevel(level)
	log.SetOutput(os.Stderr)

	if *printVersion {
		fmt.Printf("NGINX Ingress controller version: %s\n", version.Version)
		return
	}

	log.Infof("Starting NGINX Ingress controller Version %v\n", version.Version)

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

	lbc, _ := controller.NewLoadBalancerController(kubeClient, 30*time.Second, *watchNamespace, *nginxConfigMaps)
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGTERM)
	go func() {
		<-sigc
		lbc.Stop()
	}()
	lbc.Run()
}
