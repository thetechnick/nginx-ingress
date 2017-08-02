package controller

import "gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/config"

// IngressExStore returns an ingress and its related resources
type IngressExStore interface {
	GetIngressEx(ingKey string) (*config.IngressEx, error)
}

// IngressExStoreFunc wraps a closure to implement IngressExStore
type IngressExStoreFunc struct {
	getIngressEx func(ingKey string) (*config.IngressEx, error)
}

// GetIngressEx returns an ingress and its related resources
func (f *IngressExStoreFunc) GetIngressEx(ingKey string) (*config.IngressEx, error) {
	return f.getIngressEx(ingKey)
}
