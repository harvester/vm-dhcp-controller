package ippool

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	networkv1 "github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/harvester/vm-dhcp-controller/pkg/dhcp"
	"github.com/harvester/vm-dhcp-controller/pkg/util" // Added for util.ExcludedMark etc.
)

type Controller struct {
	stopCh   chan struct{}
	indexer  cache.Indexer
	queue    workqueue.RateLimitingInterface
	informer cache.Controller

	poolRef       types.NamespacedName
	dhcpAllocator *dhcp.DHCPAllocator
	poolCache     map[string]string

	initialSyncDone chan struct{}
	initialSyncOnce *sync.Once
}

func NewController(
	queue workqueue.RateLimitingInterface,
	indexer cache.Indexer,
	informer cache.Controller,
	poolRef types.NamespacedName,
	dhcpAllocator *dhcp.DHCPAllocator,
	poolCache map[string]string,
	initialSyncDone chan struct{}, // New
	initialSyncOnce *sync.Once,    // New
) *Controller {
	return &Controller{
		stopCh:          make(chan struct{}),
		informer:        informer,
		indexer:         indexer,
		queue:           queue,
		poolRef:         poolRef,
		dhcpAllocator:   dhcpAllocator,
		poolCache:       poolCache,
		initialSyncDone: initialSyncDone, // New
		initialSyncOnce: initialSyncOnce,   // New
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
			logrus.Errorf("(controller.sync) failed to update DHCP lease store for IPPool %s/%s: %s", ipPool.Namespace, ipPool.Name, err.Error())
			return err // Return error to requeue
		}
		// If update was successful, this is a good place to signal initial sync
		c.initialSyncOnce.Do(func() {
			logrus.Infof("Initial sync completed for IPPool %s/%s, signaling DHCP server.", ipPool.Namespace, ipPool.Name)
			close(c.initialSyncDone)
		})
	}

	return
}

// Update processes the IPPool and updates the DHCPAllocator's leases.
func (c *Controller) Update(ipPool *networkv1.IPPool) error {
	if ipPool == nil {
		return nil
	}

	// For simplicity, clearing existing leases for this IPPool and re-adding.
	// This assumes DHCPAllocator instance is tied to this specific IPPool processing.
	// A more robust solution might involve comparing and deleting stale leases.
	// However, DHCPAllocator's leases are keyed by MAC, so re-adding with AddLease
	// would fail if a lease for a MAC already exists due to its internal check.
	// So, we must delete first.

	// Get current leases from allocator to see which ones to delete.
	// DHCPAllocator doesn't have a "ListLeasesByNetwork" or similar.
	// And its internal `leases` map is not network-scoped.
	// This implies the agent's DHCPAllocator should only ever contain leases for THE ONE IPPool it's watching.
	// Therefore, on an IPPool update, we can iterate its *current* known leases, delete them,
	// then add all leases from the new IPPool status.

	// Step 1: Clear all leases currently in the allocator.
	// This is a simplification. A better approach would be to only remove leases
	// that are no longer in ipPool.Status.IPv4.Allocated.
	// However, DHCPAllocator doesn't provide a way to list all its MACs easily.
	// For now, we'll rely on the fact that AddLease might update if we modify it,
	// or we clear based on a local cache of what was added for this pool.
	// The `c.poolCache` (map[string]string) seems intended for this.
	// Let's assume c.poolCache stores MAC -> IP for the current IPPool.

	// currentLeasesInAllocator := make(map[string]string) // MAC -> IP // Unused

	// Let's assume `c.poolCache` stores MAC -> IP for the IPPool being managed.
	// We should clear these from dhcpAllocator first.
	for mac := range c.poolCache {
		// Check if the lease still exists in the new ipPool status. If not, delete.
		stillExists := false
		if ipPool.Status.IPv4 != nil && ipPool.Status.IPv4.Allocated != nil {
			// Check if the MAC from poolCache still exists as a value in the new ipPool.Status.IPv4.Allocated map
			for _, allocatedMacFromStatus := range ipPool.Status.IPv4.Allocated {
				if mac == allocatedMacFromStatus {
					stillExists = true
					break
				}
			}
		}
		if !stillExists {
			logrus.Infof("Deleting stale lease for MAC %s from DHCPAllocator", mac)
			if err := c.dhcpAllocator.DeleteLease(mac); err != nil {
				logrus.Warnf("Failed to delete lease for MAC %s: %v (may already be gone)", mac, err)
			}
			delete(c.poolCache, mac) // Remove from our tracking cache
		}
	}


	// Step 2: Add/Update leases from ipPool.Status.IPv4.Allocated
	if ipPool.Status.IPv4 != nil && ipPool.Status.IPv4.Allocated != nil {
		specConf := ipPool.Spec.IPv4Config
		var dnsServers []string // IPPool spec doesn't have DNS servers
		var domainName *string  // IPPool spec doesn't have domain name
		var domainSearch []string // IPPool spec doesn't have domain search
		var ntpServers []string   // IPPool spec doesn't have NTP

		for clientIPStr, hwAddr := range ipPool.Status.IPv4.Allocated {
			if hwAddr == util.ExcludedMark || hwAddr == util.ReservedMark {
				continue // Skip special markers
			}

			// If lease for this MAC already exists, delete it first to allow AddLease to work (as AddLease errors if MAC exists)
			// This handles updates to existing leases (e.g. if lease time or other params changed, though not stored in IPPool status)
			existingLease := c.dhcpAllocator.GetLease(hwAddr)
			if existingLease.ClientIP != nil { // Check if ClientIP is not nil to confirm existence
				logrus.Debugf("Deleting existing lease for MAC %s (IP: %s) before re-adding/updating.", hwAddr, existingLease.ClientIP.String())
				if err := c.dhcpAllocator.DeleteLease(hwAddr); err != nil {
					logrus.Warnf("Failed to delete existing lease for MAC %s during update: %v", hwAddr, err)
					// Continue, AddLease might still work or fail cleanly
				}
			}

			leaseTime := int(specConf.LeaseTime)
			err := c.dhcpAllocator.AddLease(
				hwAddr,
				specConf.ServerIP,
				clientIPStr,
				specConf.CIDR,
				specConf.Router,
				dnsServers,   // Empty or from a global config
				domainName,   // Nil or from a global config
				domainSearch, // Empty or from a global config
				ntpServers,   // Empty or from a global config
				&leaseTime,
			)
			if err != nil {
				logrus.Errorf("Failed to add/update lease for MAC %s, IP %s: %v", hwAddr, clientIPStr, err)
				// Potentially collect errors and return a summary
			} else {
				logrus.Infof("Successfully added/updated lease for MAC %s, IP %s", hwAddr, clientIPStr)
				c.poolCache[hwAddr] = clientIPStr // Update our tracking cache
			}
		}
	}
	logrus.Infof("DHCPAllocator cache updated for IPPool %s/%s", ipPool.Namespace, ipPool.Name)
	return nil
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
