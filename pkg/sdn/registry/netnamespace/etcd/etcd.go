package etcd

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic"
	etcdgeneric "github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/sdn/registry/netnamespace"
)

// rest implements a RESTStorage for sdn against etcd
type REST struct {
	etcdgeneric.Etcd
}

const etcdPrefix = "/registry/sdnnetnamespaces"

// NewREST returns a RESTStorage object that will work against netnamespaces
func NewREST(s storage.Interface) *REST {
	store := &etcdgeneric.Etcd{
		NewFunc:     func() runtime.Object { return &api.NetNamespace{} },
		NewListFunc: func() runtime.Object { return &api.NetNamespaceList{} },
		KeyRootFunc: func(ctx kapi.Context) string {
			return etcdPrefix
		},
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			return (etcdPrefix + "/" + name), nil
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.NetNamespace).NetName, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return netnamespace.MatchNetNamespace(label, field)
		},
		EndpointName: "netnamespace",

		Storage: s,
	}

	store.CreateStrategy = netnamespace.Strategy
	store.UpdateStrategy = netnamespace.Strategy

	return &REST{*store}
}
