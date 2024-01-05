package ippool

import (
	"context"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/workqueue"

	networkv1 "github.com/starbops/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/starbops/vm-dhcp-controller/pkg/dhcp"
	clientset "github.com/starbops/vm-dhcp-controller/pkg/generated/clientset/versioned"
	"github.com/starbops/vm-dhcp-controller/pkg/utils"
)

const (
	ADD    = "add"
	UPDATE = "update"
	DELETE = "delete"
)

type EventHandler struct {
	ctx            context.Context
	kubeConfig     string
	kubeContext    string
	kubeRestConfig *rest.Config
	k8sClientset   *clientset.Clientset

	poolRef       types.NamespacedName
	dhcpAllocator *dhcp.DHCPAllocator
	poolCache     map[string]string
}

type Event struct {
	key             string
	action          string
	poolName        string
	poolNetworkName string
}

func NewEventHandler(
	ctx context.Context,
	kubeConfig string,
	kubeContext string,
	kubeRestConfig *rest.Config,
	poolRef types.NamespacedName,
	dhcpAllocator *dhcp.DHCPAllocator,
	poolCache map[string]string,
) *EventHandler {
	return &EventHandler{
		ctx:            ctx,
		kubeConfig:     kubeConfig,
		kubeContext:    kubeContext,
		kubeRestConfig: kubeRestConfig,
		poolRef:        poolRef,
		dhcpAllocator:  dhcpAllocator,
		poolCache:      poolCache,
	}
}

func (e *EventHandler) Init() (err error) {
	e.kubeRestConfig, err = e.getKubeConfig()
	if err != nil {
		return
	}

	e.k8sClientset, err = clientset.NewForConfig(e.kubeRestConfig)
	if err != nil {
		return
	}

	return
}

func (e *EventHandler) getKubeConfig() (config *rest.Config, err error) {
	if !utils.FileExists(e.kubeConfig) {
		return rest.InClusterConfig()
	}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: e.kubeConfig},
		&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{}, CurrentContext: e.kubeContext},
	).ClientConfig()
}

func (e *EventHandler) EventListener() (err error) {
	logrus.Infof("(ippool.EventListener) starting IPPool event listener")

	// TODO: could be more specific on what namespaces we want to watch and what fields we need
	watcher := cache.NewListWatchFromClient(e.k8sClientset.NetworkV1alpha1().RESTClient(), "ippools", e.poolRef.Namespace, fields.Everything())

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	indexer, informer := cache.NewIndexerInformer(watcher, &networkv1.IPPool{}, 0, cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				queue.Add(Event{
					key:             key,
					action:          UPDATE,
					poolName:        new.(*networkv1.IPPool).ObjectMeta.Name,
					poolNetworkName: new.(*networkv1.IPPool).Spec.NetworkName,
				})
			}
		},
	}, cache.Indexers{})

	controller := NewController(queue, indexer, informer, e.poolRef, e.dhcpAllocator, e.poolCache)
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(1, stop)

	select {}
}
