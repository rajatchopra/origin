package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	sdnapi "github.com/openshift/origin/pkg/sdn/api"
)

// NetNamespaceInterface has methods to work with NetNamespace resources
type NetNamespacesInterface interface {
	NetNamespaces() NetNamespaceInterface
}

// NetNamespaceInterface exposes methods on NetNamespace resources.
type NetNamespaceInterface interface {
	List() (*sdnapi.NetNamespaceList, error)
	Get(name string) (*sdnapi.NetNamespace, error)
	Create(sub *sdnapi.NetNamespace) (*sdnapi.NetNamespace, error)
	Delete(name string) error
	Watch(resourceVersion string) (watch.Interface, error)
}

// netNamespace implements NetNamespaceInterface interface
type netNamespace struct {
	r *Client
}

// newNetNamespace returns a netnamespace
func newNetNamespace(c *Client) *netNamespace {
	return &netNamespace{
		r: c,
	}
}

// List returns a list of netnamespaces that match the label and field selectors.
func (c *netNamespace) List() (result *sdnapi.NetNamespaceList, err error) {
	result = &sdnapi.NetNamespaceList{}
	err = c.r.Get().
		Resource("netNamespaces").
		Do().
		Into(result)
	return
}

// Get returns information about a particular user or an error
func (c *netNamespace) Get(hostName string) (result *sdnapi.NetNamespace, err error) {
	result = &sdnapi.NetNamespace{}
	err = c.r.Get().Resource("netNamespaces").Name(hostName).Do().Into(result)
	return
}

// Create creates a new user. Returns the server's representation of the user and error if one occurs.
func (c *netNamespace) Create(netNamespace *sdnapi.NetNamespace) (result *sdnapi.NetNamespace, err error) {
	result = &sdnapi.NetNamespace{}
	err = c.r.Post().Resource("netNamespaces").Body(netNamespace).Do().Into(result)
	return
}

// Delete takes the name of the host, and returns an error if one occurs during deletion of the subnet
func (c *netNamespace) Delete(name string) error {
	return c.r.Delete().Resource("netNamespaces").Name(name).Do().Error()
}

// Watch returns a watch.Interface that watches the requested subnets
func (c *netNamespace) Watch(resourceVersion string) (watch.Interface, error) {
	return c.r.Get().
		Prefix("watch").
		Resource("netNamespaces").
		Param("resourceVersion", resourceVersion).
		Watch()
}
