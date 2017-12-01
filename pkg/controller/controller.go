/*
Copyright 2015 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/thetechnick/nginx-ingress/pkg/config"
	"github.com/thetechnick/nginx-ingress/pkg/errors"
	"github.com/thetechnick/nginx-ingress/pkg/storage"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	log "github.com/sirupsen/logrus"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	core_v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	api_v1 "k8s.io/client-go/pkg/api/v1"
	extensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

const (
	ingressGroupKey   = "kubernetes.io/ingress.group"
	ingressClassKey   = "kubernetes.io/ingress.class"
	nginxIngressClass = "nginx"
)

// LoadBalancerController watches Kubernetes API and
// reconfigures NGINX via Controller when needed
type LoadBalancerController struct {
	client           kubernetes.Interface
	ingController    cache.Controller
	svcController    cache.Controller
	endpController   cache.Controller
	cfgmController   cache.Controller
	secretController cache.Controller

	secretWatchlist Watchlist

	ingLister            StoreToIngressLister
	svcLister            cache.Store
	secretLister         cache.Store
	endpLister           StoreToEndpointLister
	cfgmLister           StoreToConfigMapLister
	ingQueue             TaskQueue
	cfgmQueue            TaskQueue
	stopCh               chan struct{}
	watchNginxConfigMaps bool

	configurator Configurator
}

var keyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc

// NewLoadBalancerController creates a controller
func NewLoadBalancerController(
	kubeClient kubernetes.Interface,
	resyncPeriod time.Duration,
	namespace string,
	selector labels.Selector,
	nginxConfigMaps string,
	mcs storage.MainConfigStorage,
	scs storage.ServerConfigStorage,
) (*LoadBalancerController, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&core_v1.EventSinkImpl{
		Interface: kubeClient.Core().Events(""),
	})
	lbc := LoadBalancerController{
		client:          kubeClient,
		stopCh:          make(chan struct{}),
		secretWatchlist: NewWatchlist(),
	}

	lbc.configurator = NewConfigurator(
		&ingressAccessorFuncs{lbc.getIngressByKey},
		&secretAccessorFuncs{lbc.getSecret},
		&endpointsAccessorFuncs{lbc.getEndpointsForIngressBackend},

		lbc.secretWatchlist,

		eventBroadcaster.NewRecorder(scheme.Scheme, api_v1.EventSource{Component: "ingress-controller"}),
		mcs,
		scs,
	)

	lbc.ingQueue = NewTaskQueue(lbc.syncIng, log.WithField("module", "IngressTaskQueue"))

	ingHandlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			addIng := obj.(*extensions.Ingress)
			if !isNginxIngress(addIng) {
				log.
					WithField("namespace", addIng.Namespace).
					WithField("name", addIng.Name).
					Info("Ignoring Ingress based on Annotation: %v", ingressClassKey)
				return
			}
			log.
				WithField("namespace", addIng.Namespace).
				WithField("name", addIng.Name).
				Debug("Adding Ingress")
			lbc.ingQueue.Enqueue(obj)
		},
		DeleteFunc: func(obj interface{}) {
			remIng, isIng := obj.(*extensions.Ingress)
			if !isIng {
				deletedState, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					log.
						WithField("obj", obj).
						Error("Error received unexpected ingress object, skipping")
					return
				}
				remIng, ok = deletedState.Obj.(*extensions.Ingress)
				if !ok {
					log.
						WithField("obj", deletedState.Obj).
						Error("Error DeletedFinalStateUnknown contained non-Ingress object, skipping")
					return
				}
			}
			if !isNginxIngress(remIng) {
				return
			}
			log.
				WithField("namespace", remIng.Namespace).
				WithField("name", remIng.Name).
				Debug("Removing ingress")
			lbc.ingQueue.Enqueue(obj)
		},
		UpdateFunc: func(old, cur interface{}) {
			curIng := cur.(*extensions.Ingress)
			if !isNginxIngress(curIng) {
				return
			}
			if !reflect.DeepEqual(old, cur) {
				log.
					WithField("namespace", curIng.Namespace).
					WithField("name", curIng.Name).
					Debug("Ingress changed, syncing")
				lbc.ingQueue.Enqueue(cur)
			}
		},
	}
	lbc.ingLister.Store, lbc.ingController = cache.NewInformer(
		NewListWatchFromClient(lbc.client.Extensions().RESTClient(), "ingresses", namespace, selector),
		&extensions.Ingress{}, resyncPeriod, ingHandlers)

	secretHandlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			addSecret := obj.(*api_v1.Secret)
			lbc.syncSecret(addSecret)
		},
		DeleteFunc: func(obj interface{}) {
			remSecret, isSecret := obj.(*api_v1.Secret)
			if !isSecret {
				deletedState, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					log.
						WithField("obj", obj).
						Error("Error received unexpected secret object, skipping")
					return
				}
				remSecret, ok = deletedState.Obj.(*api_v1.Secret)
				if !ok {
					log.
						WithField("obj", deletedState.Obj).
						Error("Error DeletedFinalStateUnknown contained non-Secret object, skipping")
					return
				}
			}
			lbc.syncSecret(remSecret)
		},
		UpdateFunc: func(old, cur interface{}) {
			if !reflect.DeepEqual(old, cur) {
				updateSecret := cur.(*api_v1.Secret)
				lbc.syncSecret(updateSecret)
			}
		},
	}
	lbc.secretLister, lbc.secretController = cache.NewInformer(
		cache.NewListWatchFromClient(
			lbc.client.Core().RESTClient(),
			"secrets",
			namespace,
			fields.Everything(),
		),
		&api_v1.Secret{}, resyncPeriod, secretHandlers)

	svcHandlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			addSvc := obj.(*api_v1.Service)
			log.
				WithField("namespace", addSvc.Namespace).
				WithField("name", addSvc.Name).
				Debug("Adding service")
			lbc.enqueueIngressForService(addSvc)
		},
		DeleteFunc: func(obj interface{}) {
			remSvc, isSvc := obj.(*api_v1.Service)
			if !isSvc {
				deletedState, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					log.
						WithField("obj", obj).
						Error("Error received unexpected svc object, skipping")
					return
				}
				remSvc, ok = deletedState.Obj.(*api_v1.Service)
				if !ok {
					log.
						WithField("obj", deletedState.Obj).
						Error("Error DeletedFinalStateUnknown contained non-Service object, skipping")
					return
				}
			}
			log.
				WithField("namespace", remSvc.Namespace).
				WithField("name", remSvc.Name).
				Debug("Removing service")
			lbc.enqueueIngressForService(remSvc)
		},
		UpdateFunc: func(old, cur interface{}) {
			if !reflect.DeepEqual(old, cur) {
				updateSvc := cur.(*api_v1.Service)
				log.
					WithField("namespace", updateSvc.Namespace).
					WithField("name", updateSvc.Name).
					Debug("Service changed, syncing")
				lbc.enqueueIngressForService(cur.(*api_v1.Service))
			}
		},
	}
	lbc.svcLister, lbc.svcController = cache.NewInformer(
		cache.NewListWatchFromClient(lbc.client.Core().RESTClient(), "services", namespace, fields.Everything()),
		&api_v1.Service{}, resyncPeriod, svcHandlers)

	endpHandlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			addEndp := obj.(*api_v1.Endpoints)
			log.
				WithField("namespace", addEndp.Namespace).
				WithField("name", addEndp.Name).
				Debug("Adding endpoints")
			lbc.enqueueIngressForEndpoints(addEndp)
		},
		DeleteFunc: func(obj interface{}) {
			remEndp, isEndp := obj.(*api_v1.Endpoints)
			if !isEndp {
				deletedState, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					log.
						WithField("obj", obj).
						Error("Error received unexpected endpoints object, skipping")
					return
				}
				remEndp, ok = deletedState.Obj.(*api_v1.Endpoints)
				if !ok {
					log.
						WithField("obj", deletedState.Obj).
						Error("Error DeletedFinalStateUnknown contained non-Endpoints object, skipping")
					return
				}
			}
			log.
				WithField("namespace", remEndp.Namespace).
				WithField("name", remEndp.Name).
				Debug("Removing endpoints")
			lbc.enqueueIngressForEndpoints(remEndp)
		},
		UpdateFunc: func(old, cur interface{}) {
			if !reflect.DeepEqual(old, cur) {
				endp := cur.(*api_v1.Endpoints)
				log.
					WithField("namespace", endp.Namespace).
					WithField("name", endp.Name).
					Debug("Endpoints changed, syncing")
				lbc.enqueueIngressForEndpoints(endp)
			}
		},
	}
	lbc.endpLister.Store, lbc.endpController = cache.NewInformer(
		cache.NewListWatchFromClient(lbc.client.Core().RESTClient(), "endpoints", namespace, fields.Everything()),
		&api_v1.Endpoints{}, resyncPeriod, endpHandlers)

	if nginxConfigMaps != "" {
		nginxConfigMapsNS, nginxConfigMapsName, err := parseNginxConfigMaps(nginxConfigMaps)
		if err != nil {
			log.WithError(err).Error("Invalid config-maps setting")
		} else {
			lbc.watchNginxConfigMaps = true
			lbc.cfgmQueue = NewTaskQueue(lbc.syncCfgm, log.WithField("module", "ConfigMapTaskQueue"))

			cfgmHandlers := cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					cfgm := obj.(*api_v1.ConfigMap)
					if cfgm.Name == nginxConfigMapsName {
						log.
							WithField("namespace", cfgm.Namespace).
							WithField("name", cfgm.Name).
							Debug("Adding ConfigMap")
						lbc.cfgmQueue.Enqueue(obj)
					}
				},
				DeleteFunc: func(obj interface{}) {
					cfgm, isCfgm := obj.(*api_v1.ConfigMap)
					if !isCfgm {
						deletedState, ok := obj.(cache.DeletedFinalStateUnknown)
						if !ok {
							log.
								WithField("obj", obj).
								Error("Error received unexpected ConfigMap object, skipping")
							return
						}
						cfgm, ok = deletedState.Obj.(*api_v1.ConfigMap)
						if !ok {
							log.
								WithField("obj", deletedState.Obj).
								Error("Error DeletedFinalStateUnknown contained non-ConfigMap object, skipping")
							return
						}
					}
					if cfgm.Name == nginxConfigMapsName {
						log.
							WithField("namespace", cfgm.Namespace).
							WithField("name", cfgm.Name).
							Debug("Removing ConfigMap")
						lbc.cfgmQueue.Enqueue(obj)
					}
				},
				UpdateFunc: func(old, cur interface{}) {
					if !reflect.DeepEqual(old, cur) {
						cfgm := cur.(*api_v1.ConfigMap)
						if cfgm.Name == nginxConfigMapsName {
							log.
								WithField("namespace", cfgm.Namespace).
								WithField("name", cfgm.Name).
								Debug("ConfigMap changed, syncing")
							lbc.cfgmQueue.Enqueue(cur)
						}
					}
				},
			}
			lbc.cfgmLister.Store, lbc.cfgmController = cache.NewInformer(
				cache.NewListWatchFromClient(lbc.client.Core().RESTClient(), "configmaps", nginxConfigMapsNS, fields.Everything()),
				&api_v1.ConfigMap{}, resyncPeriod, cfgmHandlers)
		}
	}

	return &lbc, nil
}

// Stop stops the loadbalancer controller
func (lbc *LoadBalancerController) Stop() {
	log.Info("Stopping gracefully")
	close(lbc.stopCh)
}

// Run starts the loadbalancer controller
func (lbc *LoadBalancerController) Run() {
	go lbc.ingController.Run(lbc.stopCh)
	go lbc.svcController.Run(lbc.stopCh)
	go lbc.endpController.Run(lbc.stopCh)
	go lbc.secretController.Run(lbc.stopCh)
	go lbc.ingQueue.Run(time.Second, lbc.stopCh)
	if lbc.watchNginxConfigMaps {
		go lbc.cfgmController.Run(lbc.stopCh)
		go lbc.cfgmQueue.Run(time.Second, lbc.stopCh)
	}
	<-lbc.stopCh
}

func (lbc *LoadBalancerController) syncSecret(secret *api_v1.Secret) {
	key, err := keyFunc(secret)
	if err != nil {
		log.WithError(err).Error("Error getting key for secret")
		return
	}
	for _, watcher := range lbc.secretWatchlist.Watchers(key) {
		lbc.ingQueue.EnqueueKey(watcher)
	}
}

func (lbc *LoadBalancerController) syncCfgm(key string) {
	nn := strings.SplitN(key, "/", 2)
	log.
		WithField("namespace", nn[0]).
		WithField("name", nn[1]).
		Debug("Syncing configmap")

	obj, cfgmExists, err := lbc.cfgmLister.GetByKey(key)
	if err != nil {
		lbc.cfgmQueue.Requeue(key, err)
		return
	}

	if !cfgmExists {
		return
	}

	// var cfg *config.Config
	cfgm := obj.(*api_v1.ConfigMap)
	if err := lbc.configurator.ConfigUpdated(cfgm); err != nil {
		lbc.cfgmQueue.Requeue(key, err)
		return
	}

	ings, _ := lbc.ingLister.List()
	for _, ing := range ings.Items {
		if !isNginxIngress(&ing) {
			continue
		}
		lbc.ingQueue.Enqueue(&ing)
	}
}

func (lbc *LoadBalancerController) syncIng(key string) {
	_, ingExists, err := lbc.ingLister.Store.GetByKey(key)
	if err != nil {
		lbc.ingQueue.Requeue(key, err)
		return
	}

	if !ingExists {
		nn := strings.SplitN(key, "/", 2)
		log.
			WithField("namespace", nn[0]).
			WithField("name", nn[1]).
			Info("Ingress no longer exists, deleting")
		if err := lbc.configurator.IngressDeleted(key); err != nil {
			lbc.ingQueue.RequeueAfter(key, err, 5*time.Second)
		}
		return
	}

	if err := lbc.configurator.IngressUpdated(key); err != nil {
		lbc.ingQueue.RequeueAfter(key, err, 5*time.Second)
		return
	}
}

func (lbc *LoadBalancerController) getIngressByKey(ingKey string) (*extensions.Ingress, error) {
	obj, ingExists, err := lbc.ingLister.Store.GetByKey(ingKey)
	if err != nil {
		return nil, err
	}
	if !ingExists {
		return nil, nil
	}

	ing := obj.(*extensions.Ingress)
	return ing, nil
}

func (lbc *LoadBalancerController) getSecret(namespace, name string) (*api_v1.Secret, error) {
	secret, err := lbc.client.Core().Secrets(namespace).Get(name, meta_v1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return secret, nil
}

func (lbc *LoadBalancerController) getIngressEx(ingKey string) (*config.IngressEx, error) {
	obj, ingExists, err := lbc.ingLister.Store.GetByKey(ingKey)
	if err != nil {
		return nil, err
	}
	if !ingExists {
		return nil, nil
	}

	ing := obj.(*extensions.Ingress)
	ingEx, err := lbc.createIngressEx(ing)
	if err != nil {
		return nil, errors.WrapInObjectContext(err, ing)
	}

	log.
		WithField("ing", ingKey).
		WithField("secrets", len(ingEx.Secrets)).
		WithField("endps", len(ingEx.Endpoints)).
		Debug("IngExStore get")

	return ingEx, nil
}

func (lbc *LoadBalancerController) enqueueIngressForService(svc *api_v1.Service) {
	ings := lbc.getIngressesForService(svc)
	for _, ing := range ings {
		if !isNginxIngress(&ing) {
			continue
		}
		lbc.ingQueue.Enqueue(&ing)
	}
}

func (lbc *LoadBalancerController) enqueueIngressForEndpoints(endp *api_v1.Endpoints) {
	ings := lbc.getIngressForEndpoints(endp)
	for _, ing := range ings {
		if !isNginxIngress(&ing) {
			continue
		}
		lbc.ingQueue.Enqueue(&ing)
	}
}

func (lbc *LoadBalancerController) getIngressesForService(svc *api_v1.Service) []extensions.Ingress {
	ings, err := lbc.ingLister.GetServiceIngress(svc)
	if err != nil {
		log.
			WithField("namespace", svc.Namespace).
			WithField("name", svc.Name).
			WithError(err).
			Debug("Ignoring Service")
		return nil
	}
	return ings
}

func (lbc *LoadBalancerController) getIngressForEndpoints(obj interface{}) []extensions.Ingress {
	var ings []extensions.Ingress
	endp := obj.(*api_v1.Endpoints)
	svcKey := endp.GetNamespace() + "/" + endp.GetName()
	svcObj, svcExists, err := lbc.svcLister.GetByKey(svcKey)
	if err != nil {
		log.
			WithField("namespace", endp.Namespace).
			WithField("name", endp.Name).
			WithError(err).
			Error("Error getting service from the cache")
	} else {
		if svcExists {
			ings = append(ings, lbc.getIngressesForService(svcObj.(*api_v1.Service))...)
		}
	}
	return ings
}

func (lbc *LoadBalancerController) createIngressEx(ing *extensions.Ingress) (*config.IngressEx, error) {
	ingEx := &config.IngressEx{
		Ingress: ing,
	}

	secrets := map[string]*api_v1.Secret{}
	for _, tls := range ing.Spec.TLS {
		secretName := tls.SecretName
		secret, err := lbc.client.Core().Secrets(ing.Namespace).Get(secretName, meta_v1.GetOptions{})
		if err != nil {
			return nil, err
		}
		secrets[secretName] = secret
	}

	ingEx.Endpoints = make(map[string][]string)
	if ing.Spec.Backend != nil {
		endps, err := lbc.getEndpointsForIngressBackend(ing.Spec.Backend, ing.Namespace)
		if err != nil {
			log.
				WithField("namespace", ing.Namespace).
				WithField("name", ing.Name).
				WithError(err).
				Errorf("Error retrieving endpoints for ingress backend: %v", ing.Spec.Backend.ServiceName)
		} else {
			ingEx.Endpoints[ing.Spec.Backend.ServiceName+ing.Spec.Backend.ServicePort.String()] = endps
		}
	}

	for _, rule := range ing.Spec.Rules {
		if rule.IngressRuleValue.HTTP == nil {
			continue
		}

		for _, path := range rule.HTTP.Paths {
			endps, err := lbc.getEndpointsForIngressBackend(&path.Backend, ing.Namespace)
			if err != nil {
				log.
					WithField("namespace", ing.Namespace).
					WithField("name", ing.Name).
					WithError(err).
					Errorf("Error retrieving endpoints for ingress backend: %v", path.Backend.ServiceName)
			} else {
				ingEx.Endpoints[path.Backend.ServiceName+path.Backend.ServicePort.String()] = endps
			}
		}
	}

	ingEx.Secrets = secrets
	return ingEx, nil
}

func (lbc *LoadBalancerController) getEndpointsForIngressBackend(backend *extensions.IngressBackend, namespace string) ([]string, error) {
	svc, err := lbc.getServiceForIngressBackend(backend, namespace)
	if err != nil {
		log.Debugf("Error getting service %v: %v", backend.ServiceName, err)
		return nil, err
	}

	endps, err := lbc.endpLister.GetServiceEndpoints(svc)
	if err != nil {
		log.Debugf("Error getting endpoints for service %s from the cache: %v", svc.Name, err)
		return nil, err
	}

	result, err := lbc.getEndpointsForPort(endps, backend.ServicePort, svc)
	if err != nil {
		log.Debugf("Error getting endpoints for service %s port %v: %v", svc.Name, backend.ServicePort, err)
		return nil, err
	}
	return result, nil
}

func (lbc *LoadBalancerController) getEndpointsForPort(endps api_v1.Endpoints, ingSvcPort intstr.IntOrString, svc *api_v1.Service) ([]string, error) {
	var targetPort int32
	var err error
	found := false

	for _, port := range svc.Spec.Ports {
		if (ingSvcPort.Type == intstr.Int && port.Port == int32(ingSvcPort.IntValue())) || (ingSvcPort.Type == intstr.String && port.Name == ingSvcPort.String()) {
			targetPort, err = lbc.getTargetPort(&port, svc)
			if err != nil {
				return nil, fmt.Errorf("Error determining target port for port %v in Ingress: %v", ingSvcPort, err)
			}
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("No port %v in service %s", ingSvcPort, svc.Name)
	}

	for _, subset := range endps.Subsets {
		for _, port := range subset.Ports {
			if port.Port == targetPort {
				var endpoints []string
				for _, address := range subset.Addresses {
					endpoint := fmt.Sprintf("%v:%v", address.IP, port.Port)
					endpoints = append(endpoints, endpoint)
				}
				return endpoints, nil
			}
		}
	}

	return nil, fmt.Errorf("No endpoints for target port %v in service %s", targetPort, svc.Name)
}

func (lbc *LoadBalancerController) getTargetPort(svcPort *api_v1.ServicePort, svc *api_v1.Service) (int32, error) {
	if (svcPort.TargetPort == intstr.IntOrString{}) {
		return svcPort.Port, nil
	}

	if svcPort.TargetPort.Type == intstr.Int {
		return int32(svcPort.TargetPort.IntValue()), nil
	}

	pods, err := lbc.client.Core().Pods(svc.Namespace).List(meta_v1.ListOptions{LabelSelector: labels.Set(svc.Spec.Selector).String()})
	if err != nil {
		return 0, fmt.Errorf("Error getting pod information: %v", err)
	}

	if len(pods.Items) == 0 {
		return 0, fmt.Errorf("No pods of service %s", svc.Name)
	}

	pod := &pods.Items[0]

	portNum, err := FindPort(pod, svcPort)
	if err != nil {
		return 0, fmt.Errorf("Error finding named port %v in pod %s: %v", svcPort, pod.Name, err)
	}

	return portNum, nil
}

func (lbc *LoadBalancerController) getServiceForIngressBackend(backend *extensions.IngressBackend, namespace string) (*api_v1.Service, error) {
	svcKey := namespace + "/" + backend.ServiceName
	svcObj, svcExists, err := lbc.svcLister.GetByKey(svcKey)
	if err != nil {
		return nil, err
	}

	if svcExists {
		return svcObj.(*api_v1.Service), nil
	}

	return nil, fmt.Errorf("service %s doesn't exists", svcKey)
}

func parseNginxConfigMaps(nginxConfigMaps string) (string, string, error) {
	res := strings.Split(nginxConfigMaps, "/")
	if len(res) != 2 {
		return "", "", fmt.Errorf("NGINX configmaps name must follow the format <namespace>/<name>, got: %v", nginxConfigMaps)
	}
	return res[0], res[1], nil
}

func isNginxIngress(ing *extensions.Ingress) bool {
	if class, exists := ing.Annotations[ingressClassKey]; exists {
		return class == nginxIngressClass || class == ""
	}

	return true
}
