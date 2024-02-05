package ippool

import (
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	networkv1 "github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/harvester/vm-dhcp-controller/pkg/dhcp"
)

type Controller struct {
	stopCh   chan struct{}
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
		stopCh:        make(chan struct{}),
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
		logrus.Errorf("(controller.sync) fetching object with key %s from store failed with %v", event.key, err)
		return
	}

	if !exists && event.action != DELETE {
		logrus.Infof("(controller.sync) IPPool %s does not exist anymore", event.key)
		return
	}

	if event.poolName != c.poolRef.Name {
		logrus.Debugf("(controller.sync) IPPool %s is not our target", event.key)
		return
	}

	switch event.action {
	case UPDATE:
		ipPool, ok := obj.(*networkv1.IPPool)
		if !ok {
			logrus.Error("(controller.sync) failed to assert obj during UPDATE")
		}
		logrus.Infof("(controller.sync) UPDATE %s/%s", ipPool.Namespace, ipPool.Name)
		if err := c.Update(ipPool); err != nil {
			logrus.Errorf("(controller.sync) failed to update DHCP lease store: %s", err.Error())
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
		logrus.Errorf("(controller.handleErr) syncing IPPool %v: %v", key, err)

		c.queue.AddRateLimited(key)

		return
	}

	c.queue.Forget(key)

	logrus.Errorf("(controller.handleErr) dropping IPPool %q out of the queue: %v", key, err)
}

func (c *Controller) Run(workers int) {
	defer runtime.HandleCrash()

	defer c.queue.ShutDown()
	logrus.Info("(controller.Run) starting IPPool controller")

	go c.informer.Run(c.stopCh)
	if !cache.WaitForCacheSync(c.stopCh, c.informer.HasSynced) {
		logrus.Errorf("(controller.Run) timed out waiting for caches to sync")

		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, c.stopCh)
	}

	<-c.stopCh

	logrus.Info("(controller.Run) IPPool controller terminated")
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}

func (c *Controller) Stop() {
	logrus.Info("(controller.Stop) stopping IPPool controller")
	close(c.stopCh)
}
