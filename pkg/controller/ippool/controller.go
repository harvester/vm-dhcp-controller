package ippool

import (
	"time"

	log "github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	kihv1 "github.com/joeyloman/kubevirt-ip-helper/pkg/apis/kubevirtiphelper.k8s.binbash.org/v1"
	kihcache "github.com/joeyloman/kubevirt-ip-helper/pkg/cache"
	"github.com/joeyloman/kubevirt-ip-helper/pkg/dhcp"
	kihclientset "github.com/joeyloman/kubevirt-ip-helper/pkg/generated/clientset/versioned"
	"github.com/joeyloman/kubevirt-ip-helper/pkg/ipam"
)

type Controller struct {
	indexer      cache.Indexer
	queue        workqueue.RateLimitingInterface
	informer     cache.Controller
	cache        *kihcache.CacheAllocator
	ipam         *ipam.IPAllocator
	dhcp         *dhcp.DHCPAllocator
	kihClientset *kihclientset.Clientset
}

func NewController(
	queue workqueue.RateLimitingInterface,
	indexer cache.Indexer,
	informer cache.Controller,
	cache *kihcache.CacheAllocator,
	ipam *ipam.IPAllocator,
	dhcp *dhcp.DHCPAllocator,
	kihClientset *kihclientset.Clientset,
) *Controller {
	return &Controller{
		informer:     informer,
		indexer:      indexer,
		queue:        queue,
		cache:        cache,
		ipam:         ipam,
		dhcp:         dhcp,
		kihClientset: kihClientset,
	}
}

func (c *Controller) processNextItem() bool {
	event, quit := c.queue.Get()
	if quit {
		return false
	}

	defer c.queue.Done(event)

	err := c.sync(event.(Event))
	c.handleErr(err, event)

	return true
}

func (c *Controller) sync(event Event) (err error) {
	obj, exists, err := c.indexer.GetByKey(event.key)
	if err != nil {
		log.Errorf("(ippool.sync) fetching object with key %s from store failed with %v", event.key, err)

		return
	}

	if !exists && event.action != DELETE {
		log.Infof("(ippool.sync) IPPool %s does not exist anymore", event.key)

		return
	}

	switch event.action {
	case ADD:
		err := c.registerIPPool(obj.(*kihv1.IPPool))
		if err != nil {
			log.Errorf("(ippool.sync) failed to allocate new pool for %s: %s",
				obj.(*kihv1.IPPool).GetName(), err.Error())

			if err := c.cleanupIPPoolObjects(obj.(kihv1.IPPool)); err != nil {
				log.Errorf("(ippool.sync) failed to cleanup pool %s: %s", event.poolName, err.Error())
			}
		}
	// case UPDATE:
	// TODO
	case DELETE:
		pool, err := c.cache.Get("pool", event.poolNetworkName)
		if err != nil {
			log.Errorf("(ippool.sync) %s", err)
		}
		if err := c.cleanupIPPoolObjects(pool.(kihv1.IPPool)); err != nil {
			log.Errorf("(ippool.sync) failed to cleanup pool %s: %s", event.poolName, err.Error())
		}
	}

	return
}

func (c *Controller) handleErr(err error, key interface{}) {
	if err == nil {
		c.queue.Forget(key)

		return
	}

	if c.queue.NumRequeues(key) < 5 {
		log.Errorf("(ippool.handleErr) syncing IPPool %v: %v", key, err)

		c.queue.AddRateLimited(key)

		return
	}

	c.queue.Forget(key)

	log.Errorf("(ippool.handleErr) dropping IPPool %q out of the queue: %v", key, err)
}

func (c *Controller) Run(workers int, stopCh chan struct{}) {
	defer runtime.HandleCrash()

	defer c.queue.ShutDown()
	log.Infof("(ippool.Run) starting IPPool controller")

	go c.informer.Run(stopCh)
	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		log.Errorf("(ippool.Run) timed out waiting for caches to sync")

		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	log.Infof("(ippool.Run) stopping IPPool controller")
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}
