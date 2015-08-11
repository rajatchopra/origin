package etcd

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic"
	etcdgeneric "github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/deploy/registry/deployconfig"
)

const DeploymentConfigPath string = "/deploymentconfigs"

type REST struct {
	*etcdgeneric.Etcd
}

// NewStorage returns a RESTStorage object that will work against DeploymentConfig objects.
func NewStorage(s storage.Interface) *REST {
	store := &etcdgeneric.Etcd{
		NewFunc:      func() runtime.Object { return &api.DeploymentConfig{} },
		NewListFunc:  func() runtime.Object { return &api.DeploymentConfigList{} },
		EndpointName: "deploymentConfig",
		KeyRootFunc: func(ctx kapi.Context) string {
			return etcdgeneric.NamespaceKeyRootFunc(ctx, DeploymentConfigPath)
		},
		KeyFunc: func(ctx kapi.Context, id string) (string, error) {
			return etcdgeneric.NamespaceKeyFunc(ctx, DeploymentConfigPath, id)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.DeploymentConfig).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return deployconfig.Matcher(label, field)
		},
		CreateStrategy:      deployconfig.Strategy,
		UpdateStrategy:      deployconfig.Strategy,
		DeleteStrategy:      deployconfig.Strategy,
		ReturnDeletedObject: false,
		Storage:             s,
	}

	return &REST{store}
}
