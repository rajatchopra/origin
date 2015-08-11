package osdn

import (
	"fmt"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	osdnapi "github.com/openshift/openshift-sdn/ovssubnet/api"

	osclient "github.com/openshift/origin/pkg/client"
	oscache "github.com/openshift/origin/pkg/client/cache"
	"github.com/openshift/origin/pkg/sdn/api"
)

type OsdnRegistryInterface struct {
	oClient osclient.Interface
	kClient kclient.Interface
}

func NewOsdnRegistryInterface(osClient *osclient.Client, kClient *kclient.Client) OsdnRegistryInterface {
	return OsdnRegistryInterface{osClient, kClient}
}

func (oi *OsdnRegistryInterface) InitSubnets() error {
	return nil
}

func (oi *OsdnRegistryInterface) GetSubnets() (*[]osdnapi.Subnet, error) {
	hostSubnetList, err := oi.oClient.HostSubnets().List()
	if err != nil {
		return nil, err
	}
	// convert HostSubnet to osdnapi.Subnet
	subList := make([]osdnapi.Subnet, 0)
	for _, subnet := range hostSubnetList.Items {
		subList = append(subList, osdnapi.Subnet{NodeIP: subnet.HostIP, Sub: subnet.Subnet})
	}
	return &subList, nil
}

func (oi *OsdnRegistryInterface) GetSubnet(nodeIP string) (*osdnapi.Subnet, error) {
	hs, err := oi.oClient.HostSubnets().Get(nodeIP)
	if err != nil {
		return nil, err
	}
	return &osdnapi.Subnet{NodeIP: hs.Host, Sub: hs.Subnet}, nil
}

func (oi *OsdnRegistryInterface) DeleteSubnet(nodeIP string) error {
	return oi.oClient.HostSubnets().Delete(nodeIP)
}

func (oi *OsdnRegistryInterface) CreateSubnet(nodeIP string, sub *osdnapi.Subnet) error {
	hs := &api.HostSubnet{
		TypeMeta:   kapi.TypeMeta{Kind: "HostSubnet"},
		ObjectMeta: kapi.ObjectMeta{Name: nodeIP},
		Host:       nodeIP,
		HostIP:     sub.NodeIP,
		Subnet:     sub.Sub,
	}
	_, err := oi.oClient.HostSubnets().Create(hs)
	return err
}

func (oi *OsdnRegistryInterface) WatchSubnets(receiver chan *osdnapi.SubnetEvent, stop chan bool) error {
	subnetEventQueue := oscache.NewEventQueue(cache.MetaNamespaceKeyFunc)
	listWatch := &cache.ListWatch{
		ListFunc: func() (runtime.Object, error) {
			return oi.oClient.HostSubnets().List()
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return oi.oClient.HostSubnets().Watch(resourceVersion)
		},
	}
	cache.NewReflector(listWatch, &api.HostSubnet{}, subnetEventQueue, 4*time.Minute).Run()

	for {
		eventType, obj, err := subnetEventQueue.Pop()
		if err != nil {
			return err
		}
		switch eventType {
		case watch.Added, watch.Modified:
			// create SubnetEvent
			hs := obj.(*api.HostSubnet)
			receiver <- &osdnapi.SubnetEvent{Type: osdnapi.Added, Node: hs.Host, Sub: osdnapi.Subnet{NodeIP: hs.HostIP, Sub: hs.Subnet}}
		case watch.Deleted:
			// TODO: There is a chance that a Delete event will not get triggered.
			// Need to use a periodic sync loop that lists and compares.
			hs := obj.(*api.HostSubnet)
			receiver <- &osdnapi.SubnetEvent{Type: osdnapi.Deleted, Node: hs.Host, Sub: osdnapi.Subnet{NodeIP: hs.HostIP, Sub: hs.Subnet}}
		}
	}
	return nil
}

func (oi *OsdnRegistryInterface) InitNodes() error {
	// return no error, as this gets initialized by apiserver
	return nil
}

func (oi *OsdnRegistryInterface) GetNodes() (*[]string, error) {
	nodes, err := oi.kClient.Nodes().List(labels.Everything(), fields.Everything())
	if err != nil {
		return nil, err
	}
	// convert kapi.NodeList to []string
	nodeList := make([]string, 0)
	for _, node := range nodes.Items {
		nodeList = append(nodeList, node.Name)
	}
	return &nodeList, nil
}

func (oi *OsdnRegistryInterface) CreateNode(nodeIP string, data string) error {
	return fmt.Errorf("Feature not supported in native mode. SDN cannot create/register nodes.")
}

func (oi *OsdnRegistryInterface) WatchNodes(receiver chan *osdnapi.NodeEvent, stop chan bool) error {
	nodeEventQueue := oscache.NewEventQueue(cache.MetaNamespaceKeyFunc)
	listWatch := &cache.ListWatch{
		ListFunc: func() (runtime.Object, error) {
			return oi.kClient.Nodes().List(labels.Everything(), fields.Everything())
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return oi.kClient.Nodes().Watch(labels.Everything(), fields.Everything(), resourceVersion)
		},
	}
	cache.NewReflector(listWatch, &kapi.Node{}, nodeEventQueue, 4*time.Minute).Run()

	for {
		eventType, obj, err := nodeEventQueue.Pop()
		if err != nil {
			return err
		}
		switch eventType {
		case watch.Added:
			// we should ignore the modified event because status updates cause unnecessary noise
			// the only time we would care about modified would be if the node changes its IP address
			// and hence all nodes need to update their vtep entries for the respective subnet
			// create nodeEvent
			node := obj.(*kapi.Node)
			receiver <- &osdnapi.NodeEvent{Type: osdnapi.Added, NodeIP: node.ObjectMeta.Name}
		case watch.Deleted:
			// TODO: There is a chance that a Delete event will not get triggered.
			// Need to use a periodic sync loop that lists and compares.
			node := obj.(*kapi.Node)
			receiver <- &osdnapi.NodeEvent{Type: osdnapi.Deleted, NodeIP: node.ObjectMeta.Name}
		}
	}
	return nil
}

func (oi *OsdnRegistryInterface) WriteNetworkConfig(network string, subnetLength uint) error {
	cn, err := oi.oClient.ClusterNetwork().Get("default")
	if err == nil {
		if cn.Network == network && cn.HostSubnetLength == int(subnetLength) {
			return nil
		} else {
			return fmt.Errorf("A network already exists and does not match the new network's parameters - Existing: (%s, %d); New: (%s, %d) ", cn.Network, cn.HostSubnetLength, network, subnetLength)
		}
	}
	cn = &api.ClusterNetwork{
		TypeMeta:         kapi.TypeMeta{Kind: "ClusterNetwork"},
		ObjectMeta:       kapi.ObjectMeta{Name: "default"},
		Network:          network,
		HostSubnetLength: int(subnetLength),
	}
	_, err = oi.oClient.ClusterNetwork().Create(cn)
	return err
}

func (oi *OsdnRegistryInterface) GetContainerNetwork() (string, error) {
	cn, err := oi.oClient.ClusterNetwork().Get("default")
	return cn.Network, err
}

func (oi *OsdnRegistryInterface) GetSubnetLength() (uint64, error) {
	cn, err := oi.oClient.ClusterNetwork().Get("default")
	return uint64(cn.HostSubnetLength), err
}

func (oi *OsdnRegistryInterface) GetServicesNetwork() (string, error) {
	// FIXME
	return "172.30.0.0/16", nil
}

func (oi *OsdnRegistryInterface) CheckEtcdIsAlive(seconds uint64) bool {
	// always assumed to be true as we run through the apiserver
	return true
}

func (oi *OsdnRegistryInterface) WatchNamespaces(receiver chan *osdnapi.NamespaceEvent, stop chan bool) error {
	nsEventQueue := oscache.NewEventQueue(cache.MetaNamespaceKeyFunc)
	listWatch := &cache.ListWatch{
		ListFunc: func() (runtime.Object, error) {
			return oi.kClient.Namespaces().List(labels.Everything(), fields.Everything())
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return oi.kClient.Namespaces().Watch(labels.Everything(), fields.Everything(), resourceVersion)
		},
	}
	cache.NewReflector(listWatch, &kapi.Namespace{}, nsEventQueue, 4*time.Minute).Run()

	for {
		eventType, obj, err := nsEventQueue.Pop()
		if err != nil {
			return err
		}
		switch eventType {
		case watch.Added:
			// we should ignore the modified event because status updates cause unnecessary noise
			// the only time we would care about modified would be if the node changes its IP address
			// and hence all nodes need to update their vtep entries for the respective subnet
			// create nodeEvent
			ns := obj.(*kapi.Namespace)
			receiver <- &osdnapi.NamespaceEvent{Type: osdnapi.Added, Name: ns.ObjectMeta.Name}
		case watch.Deleted:
			// TODO: There is a chance that a Delete event will not get triggered.
			// Need to use a periodic sync loop that lists and compares.
			ns := obj.(*kapi.Namespace)
			receiver <- &osdnapi.NamespaceEvent{Type: osdnapi.Deleted, Name: ns.ObjectMeta.Name}
		}
	}
	return nil
}

func (oi *OsdnRegistryInterface) WatchNetNamespaces(receiver chan *osdnapi.NetNamespaceEvent, stop chan bool) error {
	netNsEventQueue := oscache.NewEventQueue(cache.MetaNamespaceKeyFunc)
	listWatch := &cache.ListWatch{
		ListFunc: func() (runtime.Object, error) {
			return oi.oClient.NetNamespaces().List()
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return oi.oClient.NetNamespaces().Watch(resourceVersion)
		},
	}
	cache.NewReflector(listWatch, &api.NetNamespace{}, netNsEventQueue, 4*time.Minute).Run()

	for {
		eventType, obj, err := netNsEventQueue.Pop()
		if err != nil {
			return err
		}
		switch eventType {
		case watch.Added:
			// we should ignore the modified event because status updates cause unnecessary noise
			// the only time we would care about modified would be if the node changes its IP address
			// and hence all nodes need to update their vtep entries for the respective subnet
			// create nodeEvent
			netns := obj.(*api.NetNamespace)
			receiver <- &osdnapi.NetNamespaceEvent{Type: osdnapi.Added, Name: netns.NetName, NetID: netns.NetID}
		case watch.Deleted:
			// TODO: There is a chance that a Delete event will not get triggered.
			// Need to use a periodic sync loop that lists and compares.
			netns := obj.(*api.NetNamespace)
			receiver <- &osdnapi.NetNamespaceEvent{Type: osdnapi.Deleted, Name: netns.NetName}
		}
	}
	return nil
}

func (oi *OsdnRegistryInterface) GetNetNamespaces() ([]osdnapi.NetNamespace, error) {
	netNamespaceList, err := oi.oClient.NetNamespaces().List()
	if err != nil {
		return nil, err
	}
	// convert api.NetNamespace to osdnapi.NetNamespace
	nsList := make([]osdnapi.NetNamespace, 0)
	for _, netns := range netNamespaceList.Items {
		nsList = append(nsList, osdnapi.NetNamespace{Name: netns.Name, NetID: netns.NetID})
	}
	return nsList, nil
}

func (oi *OsdnRegistryInterface) GetNetNamespace(name string) (osdnapi.NetNamespace, error) {
	netns, err := oi.oClient.NetNamespaces().Get(name)
	if err != nil {
		return osdnapi.NetNamespace{}, err
	}
	return osdnapi.NetNamespace{Name: netns.Name, NetID: netns.NetID}, nil
}

func (oi *OsdnRegistryInterface) WriteNetNamespace(name string, id uint) error {
	netns := &api.NetNamespace{
		TypeMeta:   kapi.TypeMeta{Kind: "NetNamespace"},
		ObjectMeta: kapi.ObjectMeta{Name: name},
		NetName:    name,
		NetID:      id,
	}
	_, err := oi.oClient.NetNamespaces().Create(netns)
	return err
}

func (oi *OsdnRegistryInterface) DeleteNetNamespace(name string) error {
	return oi.oClient.NetNamespaces().Delete(name)
}

func (oi *OsdnRegistryInterface) InitServices() error {
	return nil
}

func (oi *OsdnRegistryInterface) GetServices() (*[]osdnapi.Service, error) {
	oServList := make([]osdnapi.Service, 0)
	kNsList, err := oi.kClient.Namespaces().List(labels.Everything(), fields.Everything())
	if err != nil {
		return nil, err
	}
	for _, ns := range(kNsList.Items) {
		kServList, err := oi.kClient.Services(ns.Name).List(labels.Everything())
		if err != nil {
			return nil, err
		}

		// convert kube ServiceList into []osdnapi.Service
		for _, kService := range kServList.Items {
			oService := osdnapi.Service{
				Name: kService.ObjectMeta.Name,
				Namespace: ns.Name,
				IP: kService.Spec.ClusterIP,
				Protocol: osdnapi.ServiceProtocol(kService.Spec.Ports[0].Protocol),
				Port: uint(kService.Spec.Ports[0].Port),
			}
			oServList = append(oServList, oService)
		}
	}
	return &oServList, nil
}

func (oi *OsdnRegistryInterface) WatchServices(receiver chan *osdnapi.ServiceEvent, stop chan bool) error {
	// watch for namespaces, and launch a go func for each namespace that is new
	// kill the watch for each namespace that is deleted
	nsevent := make(chan *osdnapi.NamespaceEvent)
	namespaceTable := make(map[string]chan bool)
	go oi.WatchNamespaces(nsevent, stop)
	for {
		select {
		case ev := <-nsevent:
			switch ev.Type {
			case osdnapi.Added:
				stopChannel := make(chan bool)
				namespaceTable[ev.Name] = stopChannel
				go oi.watchServicesForNamespace(ev.Name, receiver, stopChannel)
			case osdnapi.Deleted:
				stopChannel, ok := namespaceTable[ev.Name]
				if ok {
					close(stopChannel)
					delete(namespaceTable, ev.Name)
				}
			}
		case <-stop:
			// call stop on all namespace watching
			for _, stopChannel := range namespaceTable {
				close(stopChannel)
			}
			return nil
		}
	}
}

func (oi *OsdnRegistryInterface) watchServicesForNamespace(namespace string, receiver chan *osdnapi.ServiceEvent, stop chan bool) error {
	serviceEventQueue := oscache.NewEventQueue(cache.MetaNamespaceKeyFunc)
	listWatch := &cache.ListWatch{
		ListFunc: func() (runtime.Object, error) {
			return oi.kClient.Services(namespace).List(labels.Everything())
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return oi.kClient.Services(namespace).Watch(labels.Everything(), fields.Everything(), resourceVersion)
		},
	}
	cache.NewReflector(listWatch, &kapi.Service{}, serviceEventQueue, 4*time.Minute).Run()

	go func() {
		select {
		case <-stop:
			serviceEventQueue.Cancel()
		}
	}()

	for {
		eventType, obj, err := serviceEventQueue.Pop()
		if err != nil {
			if _, ok := err.(oscache.EventQueueStopped); ok {
				return nil
			}
			return err
		}
		switch eventType {
		case watch.Added:
			// we should ignore the modified event because status updates cause unnecessary noise
			// the only time we would care about modified would be if the service IP changes (does not happen)
			kServ := obj.(*kapi.Service)
			oServ := osdnapi.Service{
				Name: kServ.ObjectMeta.Name,
				Namespace: namespace,
				IP: kServ.Spec.ClusterIP,
				Protocol: osdnapi.ServiceProtocol(kServ.Spec.Ports[0].Protocol),
				Port: uint(kServ.Spec.Ports[0].Port),
			}
			receiver <- &osdnapi.ServiceEvent{Type: osdnapi.Added, Service: oServ}
		case watch.Deleted:
			// TODO: There is a chance that a Delete event will not get triggered.
			// Need to use a periodic sync loop that lists and compares.
			kServ := obj.(*kapi.Service)
			oServ := osdnapi.Service{
				Name: kServ.ObjectMeta.Name,
				Namespace: namespace,
				IP: kServ.Spec.ClusterIP,
				Protocol: osdnapi.ServiceProtocol(kServ.Spec.Ports[0].Protocol),
				Port: uint(kServ.Spec.Ports[0].Port),
			}
			receiver <- &osdnapi.ServiceEvent{Type: osdnapi.Deleted, Service: oServ}
		case watch.Error:
			// Check if the namespace is dead, if so quit
			_, err = oi.kClient.Namespaces().Get(namespace)
			if err != nil {
				break
			}
		}
	}
	return nil
}
