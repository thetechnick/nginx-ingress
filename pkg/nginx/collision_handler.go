package nginx

import (
	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/config"
	extensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

// CollisionHandler rosolves collisions in the generated configuration, so the nginx configuration stays valid.
type CollisionHandler interface {
	// AddConfigs is resolving definition collisions that would result in a invalid nginx configuration
	AddConfigs(ing *extensions.Ingress, ingServers []config.Server) (changed []config.Server, err error)

	// RemoveConfigs frees server definitions, so they can be used again
	RemoveConfigs(key string) (changed []config.Server, deleted []config.Server, err error)
}

type cacheEntry struct {
	Ingress extensions.Ingress
	Servers []config.Server
}

type cacheEntryList []cacheEntry

func (list cacheEntryList) Len() int {
	return len(list)
}
func (list cacheEntryList) Less(i, j int) bool {
	return list[i].Ingress.CreationTimestamp.Before(list[j].Ingress.CreationTimestamp)
}
func (list cacheEntryList) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}
