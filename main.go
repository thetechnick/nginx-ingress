package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/golang/glog"
	ingress_config "gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/config"
	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/controller"
	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/nginx"
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

	nginxConfigMaps = flag.String("nginx-configmaps", "",
		`Specifies a configmaps resource that can be used to customize NGINX
		configuration. The value must follow the following format: <namespace>/<name>`)

	enableMerging = flag.Bool("enable-merging", false,
		`Enables merging of ingress rules in multiple ingress objects targeting the same host.
		By default referencing the same host in multiple ingress objects will result in an error,
		and only the first ingress object will be used by the ingress controller.
		If this flag is enabled these rules will be merged using the server generated of
		the oldest ingress object as a base and adding the locations and settings of
		every ingress object in descending oder of their age.
		This is similar to the behavoir of other nginx ingress controller.`)

	printVersion = flag.Bool("version", false, "Print version and exit")
)

func main() {
	flag.Parse()
	flag.Lookup("logtostderr").Value.Set("true")

	if *printVersion {
		fmt.Printf("NGINX Ingress controller version: %s\n", version.Version)
		return
	}

	glog.Infof("Starting NGINX Ingress controller Version %v\n", version.Version)

	var err error
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
			glog.Fatalf("error creating client configuration: %v", err)
		}
	} else {
		if config, err = rest.InClusterConfig(); err != nil {
			glog.Fatalf("error creating client configuration: %v", err)
		}
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatalf("Failed to create client: %v.", err)
	}

	ngxc, _ := nginx.NewController("/etc/nginx/", *proxyURL != "", *healthStatus)
	ngxc.Start()
	nginxConfig := ingress_config.NewDefaultConfig()

	var collisionHandler nginx.CollisionHandler
	if *enableMerging {
		collisionHandler = nginx.NewMergingCollisionHandler()
	} else {
		collisionHandler = nginx.NewDenyCollisionHandler()
	}
	cnf := nginx.NewConfigurator(ngxc, collisionHandler, nginxConfig)
	lbc, _ := controller.NewLoadBalancerController(kubeClient, 30*time.Second, *watchNamespace, cnf, *nginxConfigMaps)
	lbc.Run()
}
