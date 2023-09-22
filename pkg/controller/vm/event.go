package vm

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	kubevirtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"

	kihcache "github.com/joeyloman/kubevirt-ip-helper/pkg/cache"
	"github.com/joeyloman/kubevirt-ip-helper/pkg/dhcp"
	kihclientset "github.com/joeyloman/kubevirt-ip-helper/pkg/generated/clientset/versioned"
	"github.com/joeyloman/kubevirt-ip-helper/pkg/ipam"
	"github.com/joeyloman/kubevirt-ip-helper/pkg/util"
	log "github.com/sirupsen/logrus"
)

const (
	ADD    = "add"
	UPDATE = "update"
	DELETE = "delete"
)

type EventHandler struct {
	ctx            context.Context
	ipam           *ipam.IPAllocator
	dhcp           *dhcp.DHCPAllocator
	cache          *kihcache.CacheAllocator
	kubeConfig     string
	kubeContext    string
	kubeRestConfig *rest.Config
	kihClientset   *kihclientset.Clientset
	kcli           kubecli.KubevirtClient
}

type Event struct {
	key         string
	action      string
	vmName      string
	vmNamespace string
}

func NewEventHandler(
	ctx context.Context,
	ipam *ipam.IPAllocator,
	dhcp *dhcp.DHCPAllocator,
	cache *kihcache.CacheAllocator,
	kubeConfig string,
	kubeContext string,
	kubeRestConfig *rest.Config,
	kihClientset *kihclientset.Clientset,
	kcli kubecli.KubevirtClient,
) *EventHandler {
	return &EventHandler{
		ctx:            ctx,
		ipam:           ipam,
		dhcp:           dhcp,
		cache:          cache,
		kubeConfig:     kubeConfig,
		kubeContext:    kubeContext,
		kubeRestConfig: kubeRestConfig,
		kihClientset:   kihClientset,
		kcli:           kcli,
	}
}

func (e *EventHandler) Init() (err error) {
	e.kubeRestConfig, err = e.getKubeConfig()
	if err != nil {
		return
	}

	e.kihClientset, err = kihclientset.NewForConfig(e.kubeRestConfig)
	if err != nil {
		return
	}

	e.kcli, err = kubecli.GetKubevirtClientFromRESTConfig(e.kubeRestConfig)
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
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: e.kubeConfig},
		&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{}, CurrentContext: e.kubeContext},
	).ClientConfig()
}

func (e *EventHandler) EventListener() (err error) {
	log.Infof("(vm.EventListener) starting VirtualMachine event listener")

	vmWatcher := cache.NewListWatchFromClient(e.kcli.RestClient(), "virtualmachines", corev1.NamespaceAll, fields.Everything())

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	indexer, informer := cache.NewIndexerInformer(vmWatcher, &kubevirtv1.VirtualMachine{}, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(Event{
					key:         key,
					action:      ADD,
					vmName:      obj.(*kubevirtv1.VirtualMachine).GetName(),
					vmNamespace: obj.(*kubevirtv1.VirtualMachine).GetNamespace(),
				})
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				queue.Add(Event{
					key:         key,
					action:      UPDATE,
					vmName:      new.(*kubevirtv1.VirtualMachine).GetName(),
					vmNamespace: new.(*kubevirtv1.VirtualMachine).GetNamespace(),
				})
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(Event{
					key:         key,
					action:      DELETE,
					vmName:      obj.(*kubevirtv1.VirtualMachine).GetName(),
					vmNamespace: obj.(*kubevirtv1.VirtualMachine).GetNamespace(),
				})
			}
		},
	}, cache.Indexers{})

	controller := NewController(queue, indexer, informer, e.cache, e.ipam, e.dhcp, e.kihClientset)
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(1, stop)

	select {}
}
