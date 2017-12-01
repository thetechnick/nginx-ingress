package controller

import (
	api_v1 "k8s.io/client-go/pkg/api/v1"
	extensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

// IngressAccessor contains methods to access k8s ingress objects
type IngressAccessor interface {
	GetByKey(key string) (*extensions.Ingress, error)
}

type ingressAccessorFuncs struct {
	getByKey func(key string) (*extensions.Ingress, error)
}

func (a *ingressAccessorFuncs) GetByKey(key string) (*extensions.Ingress, error) {
	return a.getByKey(key)
}

// SecretAccessor contains methods to access k8s secrets
type SecretAccessor interface {
	Get(namespace, name string) (*api_v1.Secret, error)
}

type secretAccessorFuncs struct {
	get func(namespace, name string) (*api_v1.Secret, error)
}

func (a *secretAccessorFuncs) Get(namespace, name string) (*api_v1.Secret, error) {
	return a.get(namespace, name)
}

// EndpointsAccessor contains methods to access k8s service endpoints
type EndpointsAccessor interface {
	GetEndpointsForIngressBackend(backend *extensions.IngressBackend, namespace string) ([]string, error)
}

type endpointsAccessorFuncs struct {
	getEndpointsForIngressBackend func(backend *extensions.IngressBackend, namespace string) ([]string, error)
}

func (a *endpointsAccessorFuncs) GetEndpointsForIngressBackend(backend *extensions.IngressBackend, namespace string) ([]string, error) {
	return a.getEndpointsForIngressBackend(backend, namespace)
}
