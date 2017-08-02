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

	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/config"
	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/nginx"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	log "github.com/sirupsen/logrus"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	api_v1 "k8s.io/client-go/pkg/api/v1"
	extensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

const (
	ingressClassKey   = "kubernetes.io/ingress.class"
	nginxIngressClass = "nginx"
)

// LoadBalancerController watches Kubernetes API and
// reconfigures NGINX via Controller when needed
type LoadBalancerController struct {
	client               kubernetes.Interface
	ingController        cache.Controller
	svcController        cache.Controller
	endpController       cache.Controller
	cfgmController       cache.Controller
	ingLister            StoreToIngressLister
	svcLister            cache.Store
	endpLister           StoreToEndpointLister
	cfgmLister           StoreToConfigMapLister
	ingQueue             TaskQueue
	endpQueue            TaskQueue
	cfgmQueue            TaskQueue
	stopCh               chan struct{}
	cnf                  *nginx.Configurator
	watchNginxConfigMaps bool
}

var keyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc

// NewLoadBalancerController creates a controller
func NewLoadBalancerController(kubeClient kubernetes.Interface, resyncPeriod time.Duration, namespace string, cnf *nginx.Configurator, nginxConfigMaps string) (*LoadBalancerController, error) {
	lbc := LoadBalancerController{
		client: kubeClient,
		stopCh: make(chan struct{}),
		cnf:    cnf,
	}

	lbc.ingQueue = NewTaskQueue(lbc.syncIng)
	lbc.endpQueue = NewTaskQueue(lbc.syncEndp)

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
				Info("Adding Ingress")
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
				Info("Removing ingress")
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
					Info("Ingress changed, syncing")
				lbc.ingQueue.Enqueue(cur)
			}
		},
	}
	lbc.ingLister.Store, lbc.ingController = cache.NewInformer(
		cache.NewListWatchFromClient(lbc.client.Extensions().RESTClient(), "ingresses", namespace, fields.Everything()),
		&extensions.Ingress{}, resyncPeriod, ingHandlers)

	svcHandlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			addSvc := obj.(*api_v1.Service)
			log.
				WithField("namespace", addSvc.Namespace).
				WithField("name", addSvc.Name).
				Info("Adding service")
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
				Info("Removing service")
			lbc.enqueueIngressForService(remSvc)
		},
		UpdateFunc: func(old, cur interface{}) {
			if !reflect.DeepEqual(old, cur) {
				updateSvc := cur.(*api_v1.Service)
				log.
					WithField("namespace", updateSvc.Namespace).
					WithField("name", updateSvc.Name).
					Info("Service changed, syncing")
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
				Info("Adding endpoints")
			lbc.endpQueue.Enqueue(obj)
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
				Info("Removing endpoints")
			lbc.endpQueue.Enqueue(obj)
		},
		UpdateFunc: func(old, cur interface{}) {
			if !reflect.DeepEqual(old, cur) {
				endp := cur.(*api_v1.Endpoints)
				log.
					WithField("namespace", endp.Namespace).
					WithField("name", endp.Name).
					Info("Endpoints changed, syncing")
				lbc.endpQueue.Enqueue(cur)
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
			lbc.cfgmQueue = NewTaskQueue(lbc.syncCfgm)

			cfgmHandlers := cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					cfgm := obj.(*api_v1.ConfigMap)
					if cfgm.Name == nginxConfigMapsName {
						log.
							WithField("namespace", cfgm.Namespace).
							WithField("name", cfgm.Name).
							Info("Adding ConfigMap")
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
							Info("Removing ConfigMap")
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
								Info("ConfigMap changed, syncing")
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

// Run starts the loadbalancer controller
func (lbc *LoadBalancerController) Run() {
	go lbc.ingController.Run(lbc.stopCh)
	go lbc.svcController.Run(lbc.stopCh)
	go lbc.endpController.Run(lbc.stopCh)
	go lbc.ingQueue.Run(time.Second, lbc.stopCh)
	go lbc.endpQueue.Run(time.Second, lbc.stopCh)
	if lbc.watchNginxConfigMaps {
		go lbc.cfgmController.Run(lbc.stopCh)
		go lbc.cfgmQueue.Run(time.Second, lbc.stopCh)
	}
	<-lbc.stopCh
}

func (lbc *LoadBalancerController) syncEndp(key string) {
	nn := strings.SplitN(key, "/", 2)
	log.
		WithField("namespace", nn[0]).
		WithField("name", nn[1]).
		Info("Syncing endpoints")

	obj, endpExists, err := lbc.endpLister.GetByKey(key)
	if err != nil {
		lbc.endpQueue.Requeue(key, err)
		return
	}

	if endpExists {
		ings := lbc.getIngressForEndpoints(obj)

		for _, ing := range ings {
			if !isNginxIngress(&ing) {
				continue
			}
			ingEx, err := lbc.createIngress(&ing)
			if err != nil {
				log.
					WithField("namespace", ing.Namespace).
					WithField("name", ing.Name).
					Info("Error updating endpoints for Ingress")
				continue
			}

			log.
				WithField("namespace", ing.Namespace).
				WithField("name", ing.Name).
				Info("Updating Endpoints for Ingress")
			name := ing.Namespace + "-" + ing.Name
			lbc.cnf.UpdateEndpoints(name, ingEx)
		}
	}

}

func (lbc *LoadBalancerController) syncCfgm(key string) {
	nn := strings.SplitN(key, "/", 2)
	log.
		WithField("namespace", nn[0]).
		WithField("name", nn[1]).
		Info("Syncing configmap")

	obj, cfgmExists, err := lbc.cfgmLister.GetByKey(key)
	if err != nil {
		lbc.cfgmQueue.Requeue(key, err)
		return
	}
	cfg := config.NewDefaultConfig()

	if cfgmExists {
		cfgm := obj.(*api_v1.ConfigMap)

		if serverTokens, exists, err := nginx.GetMapKeyAsBool(cfgm.Data, "server-tokens", cfgm); exists {
			if err != nil {
				log.
					WithField("namespace", cfgm.Namespace).
					WithField("name", cfgm.Name).
					WithField("key", "server-tokens").
					WithError(err).
					Error("Error parsing ConfigMap, skipping key")
			} else {
				cfg.ServerTokens = serverTokens
			}
		}

		if proxyConnectTimeout, exists := cfgm.Data["proxy-connect-timeout"]; exists {
			cfg.ProxyConnectTimeout = proxyConnectTimeout
		}
		if proxyReadTimeout, exists := cfgm.Data["proxy-read-timeout"]; exists {
			cfg.ProxyReadTimeout = proxyReadTimeout
		}
		if proxyHideHeaders, exists, err := nginx.GetMapKeyAsStringSlice(cfgm.Data, "proxy-hide-headers", cfgm, ","); exists {
			if err != nil {
				log.
					WithField("namespace", cfgm.Namespace).
					WithField("name", cfgm.Name).
					WithField("key", "proxy-hide-headers").
					WithError(err).
					Error("Error parsing ConfigMap, skipping key")
			} else {
				cfg.ProxyHideHeaders = proxyHideHeaders
			}
		}
		if proxyPassHeaders, exists, err := nginx.GetMapKeyAsStringSlice(cfgm.Data, "proxy-pass-headers", cfgm, ","); exists {
			if err != nil {
				log.
					WithField("namespace", cfgm.Namespace).
					WithField("name", cfgm.Name).
					WithField("key", "proxy-pass-headers").
					WithError(err).
					Error("Error validating ConfigMap, skipping key")
			} else {
				cfg.ProxyPassHeaders = proxyPassHeaders
			}
		}
		if clientMaxBodySize, exists := cfgm.Data["client-max-body-size"]; exists {
			cfg.ClientMaxBodySize = clientMaxBodySize
		}
		if serverNamesHashBucketSize, exists := cfgm.Data["server-names-hash-bucket-size"]; exists {
			cfg.MainServerNamesHashBucketSize = serverNamesHashBucketSize
		}
		if serverNamesHashMaxSize, exists := cfgm.Data["server-names-hash-max-size"]; exists {
			cfg.MainServerNamesHashMaxSize = serverNamesHashMaxSize
		}
		if HTTP2, exists, err := nginx.GetMapKeyAsBool(cfgm.Data, "http2", cfgm); exists {
			if err != nil {
				log.
					WithField("namespace", cfgm.Namespace).
					WithField("name", cfgm.Name).
					WithField("key", "http2").
					WithError(err).
					Error("Error validating ConfigMap, skipping key")
			} else {
				cfg.HTTP2 = HTTP2
			}
		}
		if redirectToHTTPS, exists, err := nginx.GetMapKeyAsBool(cfgm.Data, "redirect-to-https", cfgm); exists {
			if err != nil {
				log.
					WithField("namespace", cfgm.Namespace).
					WithField("name", cfgm.Name).
					WithField("key", "redirect-to-https").
					WithError(err).
					Error("Error validating ConfigMap, skipping key")
			} else {
				cfg.RedirectToHTTPS = redirectToHTTPS
			}
		}

		// HSTS block
		if hsts, exists, err := nginx.GetMapKeyAsBool(cfgm.Data, "hsts", cfgm); exists {
			if err != nil {
				log.
					WithField("namespace", cfgm.Namespace).
					WithField("name", cfgm.Name).
					WithField("key", "hsts").
					WithError(err).
					Error("Error validating ConfigMap, skipping key")
			} else {
				parsingErrors := false

				hstsMaxAge, existsMA, err := nginx.GetMapKeyAsInt(cfgm.Data, "hsts-max-age", cfgm)
				if existsMA && err != nil {
					log.
						WithField("namespace", cfgm.Namespace).
						WithField("name", cfgm.Name).
						WithField("key", "hsts-max-age").
						WithError(err).
						Error("Error validating ConfigMap, skipping key")
					parsingErrors = true
				}
				hstsIncludeSubdomains, existsIS, err := nginx.GetMapKeyAsBool(cfgm.Data, "hsts-include-subdomains", cfgm)
				if existsIS && err != nil {
					log.
						WithField("namespace", cfgm.Namespace).
						WithField("name", cfgm.Name).
						WithField("key", "hsts-include-subdomains").
						WithError(err).
						Error("Error validating ConfigMap, skipping key")
					parsingErrors = true
				}

				if parsingErrors {
					log.
						WithField("namespace", cfgm.Namespace).
						WithField("name", cfgm.Name).
						WithError(err).
						Error("Error validating HSTS settings in ConfigMap, skipping keys for all hsts settings")
				} else {
					cfg.HSTS = hsts
					if existsMA {
						cfg.HSTSMaxAge = hstsMaxAge
					}
					if existsIS {
						cfg.HSTSIncludeSubdomains = hstsIncludeSubdomains
					}
				}
			}
		}

		if proxyProtocol, exists, err := nginx.GetMapKeyAsBool(cfgm.Data, "proxy-protocol", cfgm); exists {
			if err != nil {
				log.
					WithField("namespace", cfgm.Namespace).
					WithField("name", cfgm.Name).
					WithField("key", "proxy-protocol").
					WithError(err).
					Error("Error validating ConfigMap, skipping key")
			} else {
				cfg.ProxyProtocol = proxyProtocol
			}
		}

		// ngx_http_realip_module
		if realIPHeader, exists := cfgm.Data["real-ip-header"]; exists {
			cfg.RealIPHeader = realIPHeader
		}
		if setRealIPFrom, exists, err := nginx.GetMapKeyAsStringSlice(cfgm.Data, "set-real-ip-from", cfgm, ","); exists {
			if err != nil {
				log.
					WithField("namespace", cfgm.Namespace).
					WithField("name", cfgm.Name).
					WithField("key", "set-real-ip-from").
					WithError(err).
					Error("Error validating ConfigMap, skipping key")
			} else {
				cfg.SetRealIPFrom = setRealIPFrom
			}
		}
		if realIPRecursive, exists, err := nginx.GetMapKeyAsBool(cfgm.Data, "real-ip-recursive", cfgm); exists {
			if err != nil {
				log.
					WithField("namespace", cfgm.Namespace).
					WithField("name", cfgm.Name).
					WithField("key", "real-ip-recursive").
					WithError(err).
					Error("Error validating ConfigMap, skipping key")
			} else {
				cfg.RealIPRecursive = realIPRecursive
			}
		}

		// SSL block
		if sslProtocols, exists := cfgm.Data["ssl-protocols"]; exists {
			cfg.MainServerSSLProtocols = sslProtocols
		}
		if sslPreferServerCiphers, exists, err := nginx.GetMapKeyAsBool(cfgm.Data, "ssl-prefer-server-ciphers", cfgm); exists {
			if err != nil {
				log.
					WithField("namespace", cfgm.Namespace).
					WithField("name", cfgm.Name).
					WithField("key", "ssl-prefer-server-ciphers").
					WithError(err).
					Error("Error validating ConfigMap, skipping key")
			} else {
				cfg.MainServerSSLPreferServerCiphers = sslPreferServerCiphers
			}
		}
		if sslCiphers, exists := cfgm.Data["ssl-ciphers"]; exists {
			cfg.MainServerSSLCiphers = strings.Trim(sslCiphers, "\n")
		}
		if sslDHParamFile, exists := cfgm.Data["ssl-dhparam-file"]; exists {
			sslDHParamFile = strings.Trim(sslDHParamFile, "\n")
			fileName, err := lbc.cnf.AddOrUpdateDHParam(sslDHParamFile)
			if err != nil {
				log.
					WithField("namespace", cfgm.Namespace).
					WithField("name", cfgm.Name).
					WithField("key", "ssl-dhparam-file").
					WithError(err).
					Error("Error updating dhparams from ConfigMap, skipping key")
			} else {
				cfg.MainServerSSLDHParam = fileName
			}
		}

		if logFormat, exists := cfgm.Data["log-format"]; exists {
			cfg.MainLogFormat = logFormat
		}
		if proxyBuffering, exists, err := nginx.GetMapKeyAsBool(cfgm.Data, "proxy-buffering", cfgm); exists {
			if err != nil {
				log.
					WithField("namespace", cfgm.Namespace).
					WithField("name", cfgm.Name).
					WithField("key", "proxy-buffering").
					WithError(err).
					Error("Error validating ConfigMap, skipping key")
			} else {
				cfg.ProxyBuffering = proxyBuffering
			}
		}
		if proxyBuffers, exists := cfgm.Data["proxy-buffers"]; exists {
			cfg.ProxyBuffers = proxyBuffers
		}
		if proxyBufferSize, exists := cfgm.Data["proxy-buffer-size"]; exists {
			cfg.ProxyBufferSize = proxyBufferSize
		}
		if proxyMaxTempFileSize, exists := cfgm.Data["proxy-max-temp-file-size"]; exists {
			cfg.ProxyMaxTempFileSize = proxyMaxTempFileSize
		}

		if mainHTTPSnippets, exists, err := nginx.GetMapKeyAsStringSlice(cfgm.Data, "http-snippets", cfgm, "\n"); exists {
			if err != nil {
				log.
					WithField("namespace", cfgm.Namespace).
					WithField("name", cfgm.Name).
					WithField("key", "http-snippets").
					WithError(err).
					Error("Error validating ConfigMap, skipping key")
			} else {
				cfg.MainHTTPSnippets = mainHTTPSnippets
			}
		}
		if locationSnippets, exists, err := nginx.GetMapKeyAsStringSlice(cfgm.Data, "location-snippets", cfgm, "\n"); exists {
			if err != nil {
				log.
					WithField("namespace", cfgm.Namespace).
					WithField("name", cfgm.Name).
					WithField("key", "location-snippets").
					WithError(err).
					Error("Error validating ConfigMap, skipping key")
			} else {
				cfg.LocationSnippets = locationSnippets
			}
		}
		if serverSnippets, exists, err := nginx.GetMapKeyAsStringSlice(cfgm.Data, "server-snippets", cfgm, "\n"); exists {
			if err != nil {
				log.
					WithField("namespace", cfgm.Namespace).
					WithField("name", cfgm.Name).
					WithField("key", "server-snippets").
					WithError(err).
					Error("Error validating ConfigMap, skipping key")
			} else {
				cfg.ServerSnippets = serverSnippets
			}
		}

	}
	lbc.cnf.UpdateConfig(cfg)

	ings, _ := lbc.ingLister.List()
	for _, ing := range ings.Items {
		if !isNginxIngress(&ing) {
			continue
		}
		lbc.ingQueue.Enqueue(&ing)
	}
}

func (lbc *LoadBalancerController) syncIng(key string) {
	nn := strings.SplitN(key, "/", 2)
	log.
		WithField("namespace", nn[0]).
		WithField("name", nn[1]).
		Info("Syncing Ingress")

	obj, ingExists, err := lbc.ingLister.Store.GetByKey(key)
	if err != nil {
		lbc.ingQueue.Requeue(key, err)
		return
	}

	// defaut/some-ingress -> default-some-ingress
	name := strings.Replace(key, "/", "-", -1)

	if !ingExists {
		log.
			WithField("namespace", nn[0]).
			WithField("name", nn[1]).
			Info("Deleting Ingress")
		lbc.cnf.DeleteIngress(name)
	} else {
		log.
			WithField("namespace", nn[0]).
			WithField("name", nn[1]).
			Info("Adding or Updating Ingress")
		ing := obj.(*extensions.Ingress)
		ingEx, err := lbc.createIngress(ing)
		if err != nil {
			lbc.ingQueue.RequeueAfter(key, err, 5*time.Second)
			return
		}
		lbc.cnf.AddOrUpdateIngress(name, ingEx)
	}
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

func (lbc *LoadBalancerController) getIngressesForService(svc *api_v1.Service) []extensions.Ingress {
	ings, err := lbc.ingLister.GetServiceIngress(svc)
	if err != nil {
		log.
			WithField("namespace", svc.Namespace).
			WithField("name", svc.Name).
			WithError(err).
			Info("Ignoring Service")
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

func (lbc *LoadBalancerController) createIngress(ing *extensions.Ingress) (*nginx.IngressEx, error) {
	ingEx := &nginx.IngressEx{
		Ingress: ing,
	}

	ingEx.Secrets = make(map[string]*api_v1.Secret)
	for _, tls := range ing.Spec.TLS {
		secretName := tls.SecretName
		secret, err := lbc.client.Core().Secrets(ing.Namespace).Get(secretName, meta_v1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("Error retrieving secret %v for Ingress %v: %v", secretName, ing.Name, err)
		}
		ingEx.Secrets[secretName] = secret
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
					Errorf("Error retrieving endpoints for ingress backend: %v", ing.Spec.Backend.ServiceName)
			} else {
				ingEx.Endpoints[path.Backend.ServiceName+path.Backend.ServicePort.String()] = endps
			}
		}
	}

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
