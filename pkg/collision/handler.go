package collision

import (
	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/config"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

// Handler resolves collisions of generated servers
type Handler interface {
	Resolve(mergeList MergeList) (updated []MergedIngressConfig, err error)
}

type collisionContext struct {
	Ingress v1beta1.Ingress
	Servers []config.Server
}

type collisionContextList []collisionContext

func (list collisionContextList) Len() int {
	return len(list)
}
func (list collisionContextList) Less(i, j int) bool {
	return list[i].Ingress.CreationTimestamp.Before(list[j].Ingress.CreationTimestamp)
}
func (list collisionContextList) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}

// IngressConfig ties together the generated server configs and the ingress they where created from
type IngressConfig struct {
	Ingress *v1beta1.Ingress
	Servers []*config.Server
}

// MergeList is a list of configs that need to be merged
type MergeList []IngressConfig

func (list MergeList) Len() int {
	return len(list)
}
func (list MergeList) Less(i, j int) bool {
	return list[i].Ingress.CreationTimestamp.Before(list[j].Ingress.CreationTimestamp)
}
func (list MergeList) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}

// MergedIngressConfig is the result of merging the server configs of multiple ingress object into one
type MergedIngressConfig struct {
	Ingress []*v1beta1.Ingress
	Server  *config.Server
}
