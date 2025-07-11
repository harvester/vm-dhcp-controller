package ippool

import (
	"context"
	"sync"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/workqueue"

	networkv1 "github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/harvester/vm-dhcp-controller/pkg/dhcp"
	clientset "github.com/harvester/vm-dhcp-controller/pkg/generated/clientset/versioned"
	"github.com/harvester/vm-dhcp-controller/pkg/util"
)

const (
	ADD    = "add"
	UPDATE = "update"
	DELETE = "delete"
)

type EventHandler struct {
	kubeConfig     string
	kubeContext    string
	kubeRestConfig *rest.Config
	k8sClientset   *clientset.Clientset

	poolRef       types.NamespacedName
	dhcpAllocator *dhcp.DHCPAllocator
	poolCache     map[string]string

	InitialSyncDone chan struct{}
	initialSyncOnce sync.Once
}

// GetPoolRef returns the string representation of the IPPool an EventHandler is responsible for.
func (e *EventHandler) GetPoolRef() types.NamespacedName {
	return e.poolRef
}

type Event struct {
	key             string
	action          string
	poolName        string
	poolNetworkName string
}

func NewEventHandler(
	kubeConfig string,
	kubeContext string,
	kubeRestConfig *rest.Config,
	poolRef types.NamespacedName,
	dhcpAllocator *dhcp.DHCPAllocator,
	poolCache map[string]string,
) *EventHandler {
	return &EventHandler{
		kubeConfig:     kubeConfig,
		kubeContext:    kubeContext,
		kubeRestConfig: kubeRestConfig,
		poolRef:        poolRef,
		dhcpAllocator:  dhcpAllocator,
		poolCache:      poolCache,
		InitialSyncDone: make(chan struct{}),
		// initialSyncOnce is zero-valued sync.Once, which is ready to use
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
	if !util.FileExists(e.kubeConfig) {
		return rest.InClusterConfig()
	}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{
			ExplicitPath: e.kubeConfig,
		},
		&clientcmd.ConfigOverrides{
			ClusterInfo:    clientcmdapi.Cluster{},
			CurrentContext: e.kubeContext,
		},
	).ClientConfig()
}

func (e *EventHandler) EventListener(ctx context.Context) {
	logrus.Info("(eventhandler.EventListener) starting IPPool event listener")

	// TODO: could be more specific on what namespaces we want to watch and what fields we need
	// Watch only the specific IPPool this EventHandler is responsible for.
	nameSelector := fields.OneTermEqualSelector("metadata.name", e.poolRef.Name)
	watcher := cache.NewListWatchFromClient(e.k8sClientset.NetworkV1alpha1().RESTClient(), "ippools", e.poolRef.Namespace, nameSelector)

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	indexer, informer := cache.NewIndexerInformer(watcher, &networkv1.IPPool{}, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				ipPool := obj.(*networkv1.IPPool)
				// Ensure we only queue events for the specific IPPool this handler is for,
				// even though the watcher is now scoped. This is a good safeguard.
				if ipPool.Name == e.poolRef.Name && ipPool.Namespace == e.poolRef.Namespace {
					queue.Add(Event{
						key:             key,
						action:          ADD,
						poolName:        ipPool.ObjectMeta.Name,
						poolNetworkName: ipPool.Spec.NetworkName,
					})
				}
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				ipPool := new.(*networkv1.IPPool)
				// Ensure we only queue events for the specific IPPool this handler is for.
				if ipPool.Name == e.poolRef.Name && ipPool.Namespace == e.poolRef.Namespace {
					queue.Add(Event{
						key:             key,
						action:          UPDATE,
						poolName:        ipPool.ObjectMeta.Name,
						poolNetworkName: ipPool.Spec.NetworkName,
					})
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj) // Important for handling deleted objects
			if err == nil {
				var poolName, poolNamespace string
				// Attempt to get name and namespace from the object if possible
				if ipPool, ok := obj.(*networkv1.IPPool); ok {
					poolName = ipPool.ObjectMeta.Name
					poolNamespace = ipPool.ObjectMeta.Namespace
				} else if dslu, ok := obj.(cache.DeletedFinalStateUnknown); ok {
					// Try to get original object
					if ipPoolOrig, okOrig := dslu.Obj.(*networkv1.IPPool); okOrig {
						poolName = ipPoolOrig.ObjectMeta.Name
						poolNamespace = ipPoolOrig.ObjectMeta.Namespace
					} else { // Fallback to splitting the key
						ns, name, keyErr := cache.SplitMetaNamespaceKey(key)
						if keyErr == nil {
							poolName = name
							poolNamespace = ns
						}
					}
				} else { // Fallback to splitting the key if obj is not IPPool or DeletedFinalStateUnknown
					ns, name, keyErr := cache.SplitMetaNamespaceKey(key)
					if keyErr == nil {
						poolName = name
						poolNamespace = ns
					}
				}

				// Ensure we only queue events for the specific IPPool this handler is for.
				if poolName == e.poolRef.Name && poolNamespace == e.poolRef.Namespace {
					// For DELETE, poolNetworkName might not be available or relevant in the Event struct
					// if the controller's delete logic primarily uses the key/poolRef.
					queue.Add(Event{
						key:      key,
						action:   DELETE,
						poolName: poolName,
						// poolNetworkName could be omitted or fetched if truly needed for DELETE logic
					})
				}
			}
		},
	}, cache.Indexers{})

	controller := NewController(queue, indexer, informer, e.poolRef, e.dhcpAllocator, e.poolCache, e.InitialSyncDone, &e.initialSyncOnce)

	go controller.Run(1)

	<-ctx.Done()
	controller.Stop()

	logrus.Info("(eventhandler.Run) IPPool event listener terminated")
}
