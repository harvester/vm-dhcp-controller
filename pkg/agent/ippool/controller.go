package ippool

import (
	"time"

	"github.com/sirupsen/logrus"
	networkv1 "github.com/starbops/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/starbops/vm-dhcp-controller/pkg/dhcp"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type Controller struct {
	indexer  cache.Indexer
	queue    workqueue.RateLimitingInterface
	informer cache.Controller

	poolRef       types.NamespacedName
	dhcpAllocator *dhcp.DHCPAllocator
	poolCache     map[string]string
}

func NewController(
	queue workqueue.RateLimitingInterface,
	indexer cache.Indexer,
	informer cache.Controller,
	poolRef types.NamespacedName,
	dhcpAllocator *dhcp.DHCPAllocator,
	poolCache map[string]string,
) *Controller {
	return &Controller{
		informer:      informer,
		indexer:       indexer,
		queue:         queue,
		poolRef:       poolRef,
		dhcpAllocator: dhcpAllocator,
		poolCache:     poolCache,
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
		logrus.Errorf("(ippool.sync) fetching object with key %s from store failed with %v", event.key, err)
		return
	}

	if !exists && event.action != DELETE {
		logrus.Infof("(ippool.sync) IPPool %s does not exist anymore", event.key)
		return
	}

	if event.poolName != c.poolRef.Name {
		logrus.Infof("(ippool.sync) IPPool %s is not our target", event.key)
		return
	}

	switch event.action {
	case UPDATE:
		ipPool, ok := obj.(*networkv1.IPPool)
		if !ok {
			logrus.Error("(ippool.sync) failed to assert obj during UPDATE")
		}
		logrus.Infof("(ippool.sync) UPDATE %s/%s", ipPool.Namespace, ipPool.Name)
		if err := c.Update(ipPool); err != nil {
			logrus.Errorf("(ippool.sync) failed to update DHCP lease store")
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
		logrus.Errorf("(ippool.handleErr) syncing IPPool %v: %v", key, err)

		c.queue.AddRateLimited(key)

		return
	}

	c.queue.Forget(key)

	logrus.Errorf("(ippool.handleErr) dropping IPPool %q out of the queue: %v", key, err)
}

func (c *Controller) Run(workers int, stopCh chan struct{}) {
	defer runtime.HandleCrash()

	defer c.queue.ShutDown()
	logrus.Infof("(ippool.Run) starting IPPool controller")

	go c.informer.Run(stopCh)
	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		logrus.Errorf("(ippool.Run) timed out waiting for caches to sync")

		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	logrus.Infof("(ippool.Run) stopping IPPool controller")
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}
