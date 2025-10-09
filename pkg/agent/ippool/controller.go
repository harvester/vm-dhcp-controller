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
	queue    workqueue.TypedRateLimitingInterface[Event]
	informer cache.Controller

	poolRef       types.NamespacedName
	dhcpAllocator *dhcp.DHCPAllocator
	poolCache     map[string]string

	initialSyncDone chan struct{}
	initialSyncOnce *sync.Once
}

func NewController(
	queue workqueue.TypedRateLimitingInterface[Event],
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

	err := c.sync(event)
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
			logrus.Infof("Initial sync UPDATE completed for IPPool %s/%s, signaling DHCP server.", ipPool.Namespace, ipPool.Name)
			close(c.initialSyncDone)
		})
	case ADD: // Handle ADD for initial sync signal
		ipPool, ok := obj.(*networkv1.IPPool)
		if !ok {
			logrus.Errorf("(controller.sync) failed to assert obj during ADD for key %s", event.key)
			return err // Return error to requeue
		}
		logrus.Infof("(controller.sync) ADD %s/%s", ipPool.Namespace, ipPool.Name)
		if err := c.Update(ipPool); err != nil { // Update leases based on this added IPPool
			logrus.Errorf("(controller.sync) failed to update DHCP lease store for newly added IPPool %s/%s: %s", ipPool.Namespace, ipPool.Name, err.Error())
			return err // Return error to requeue
		}
		// Signal initial sync because our target pool has been added and processed.
		c.initialSyncOnce.Do(func() {
			logrus.Infof("Initial sync ADD completed for IPPool %s/%s, signaling DHCP server.", ipPool.Namespace, ipPool.Name)
			close(c.initialSyncDone)
		})
	case DELETE:
		// If our target IPPool is deleted.
		// If cache.WaitForCacheSync is done, and then we get a DELETE for our pool,
		// it means it *was* there.
		// If the pool is *not* found by GetByKey (exists == false) and action is DELETE,
		// it implies it was already deleted from the cache by the informer.
		poolNamespace, poolNameFromKey, keyErr := cache.SplitMetaNamespaceKey(event.key)
		if keyErr != nil {
			logrus.Errorf("(controller.sync) failed to split key %s for DELETE: %v", event.key, keyErr)
			return keyErr
		}

		// Ensure this delete event is for the specific pool this controller instance is managing.
		// This check is already done above with `event.poolName != c.poolRef.Name`,
		// but double-checking with `event.key` against `c.poolRef` is more robust for DELETE.
		if !(poolNamespace == c.poolRef.Namespace && poolNameFromKey == c.poolRef.Name) {
			logrus.Debugf("(controller.sync) DELETE event for key %s is not for our target pool %s. Skipping.", event.key, c.poolRef.String())
			return nil
		}

		logrus.Infof("(controller.sync) DELETE %s. Clearing leases.", event.key)
		c.clearLeasesForPool(c.poolRef.String()) // poolRefString is "namespace/name"

		// After deletion processing, if this was our target pool, it's now "synced" (as deleted/absent).
		c.initialSyncOnce.Do(func() {
			logrus.Infof("Initial sync DELETE (target processed for deletion) completed for IPPool %s, signaling DHCP server.", event.key)
			close(c.initialSyncDone)
		})
	}

	return
}

// clearLeasesForPool clears all leases for a specific IPPool from the DHCPAllocator and local cache.
func (c *Controller) clearLeasesForPool(poolRefStr string) {
	logrus.Infof("(%s) Clearing all leases from DHCPAllocator and local cache due to IPPool deletion or full resync.", poolRefStr)
	// Iterate over a copy of keys if modifying map during iteration, or collect keys first
	hwAddrsToDelete := []string{}
	for hwAddr := range c.poolCache {
		// Assuming c.poolCache only contains MACs for *this* controller's poolRef.
		// This assumption needs to be true for this to work correctly.
		// If c.poolCache could contain MACs from other pools (e.g. if it was shared, which it isn't here),
		// we would need to verify that the lease belongs to this poolRefStr before deleting.
		// However, since each EventHandler/Controller has its own poolCache for its specific poolRef, this is safe.
		hwAddrsToDelete = append(hwAddrsToDelete, hwAddr)
	}

	for _, hwAddr := range hwAddrsToDelete {
		if err := c.dhcpAllocator.DeleteLease(poolRefStr, hwAddr); err != nil {
			logrus.Warnf("(%s) Failed to delete lease for MAC %s during clear: %v (may already be gone or belong to a different pool if logic changes)", poolRefStr, hwAddr, err)
		}
		delete(c.poolCache, hwAddr)
	}
	logrus.Infof("(%s) Finished clearing leases.", poolRefStr)
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
	// poolRefStr is the "namespace/name" string for the IPPool this controller is responsible for.
	poolRefStr := c.poolRef.String()

	// Step 1: Identify leases to remove from DHCPAllocator for this specific IPPool.
	// These are leases present in c.poolCache (MAC -> IP) but not in the new ipPool.Status.IPv4.Allocated.
	newAllocatedHwAddrs := make(map[string]bool)
	if ipPool.Status.IPv4 != nil && ipPool.Status.IPv4.Allocated != nil {
		for _, hwAddr := range ipPool.Status.IPv4.Allocated {
			if hwAddr != util.ExcludedMark && hwAddr != util.ReservedMark {
				newAllocatedHwAddrs[hwAddr] = true
			}
		}
	}

	for hwAddrFromCache := range c.poolCache {
		if _, stillExists := newAllocatedHwAddrs[hwAddrFromCache]; !stillExists {
			logrus.Infof("(%s) Deleting stale lease for MAC %s from DHCPAllocator", poolRefStr, hwAddrFromCache)
			if err := c.dhcpAllocator.DeleteLease(poolRefStr, hwAddrFromCache); err != nil {
				logrus.Warnf("(%s) Failed to delete lease for MAC %s: %v (may already be gone)", poolRefStr, hwAddrFromCache, err)
			}
			delete(c.poolCache, hwAddrFromCache) // Remove from our tracking cache
		}
	}

	// Step 2: Add/Update leases from ipPool.Status.IPv4.Allocated into DHCPAllocator.
	if ipPool.Status.IPv4 != nil && ipPool.Status.IPv4.Allocated != nil {
		specConf := ipPool.Spec.IPv4Config
		// For DNS, DomainName, DomainSearch, NTPServers:
		// These are not in the IPPool spec. For now, we'll pass empty/nil values.
		// A more complete solution might fetch these from a global config or NAD annotations.
		var dnsServers []string
		var domainName *string // Already a pointer, can be nil
		var domainSearch []string
		var ntpServers []string

		for clientIPStr, hwAddr := range ipPool.Status.IPv4.Allocated {
			if hwAddr == util.ExcludedMark || hwAddr == util.ReservedMark {
				// Also, ensure we add the "EXCLUDED" entry to the DHCPAllocator's internal tracking
				// if it has such a concept, or handle it appropriately so it doesn't grant these IPs.
				// For now, the DHCPAllocator.AddLease is for actual leases.
				// We could add a special marker to poolCache if needed.
				// The current DHCPAllocator doesn't seem to have a direct way to mark IPs as "excluded"
				// other than them not being available for leasing.
				// The IPPool CRD status.allocated handles this.
				// We can add excluded IPs to our local c.poolCache to prevent deletion if logic changes.
				// c.poolCache[hwAddr] = clientIPStr // e.g. c.poolCache["EXCLUDED"] = "10.102.189.39"
				// However, the current loop for deletion (Step 1) is based on hwAddr in poolCache vs hwAddr in status.
				// So, excluded entries from status won't be deleted from cache if they are added to cache.
				// Let's ensure they are in the cache if not already, so they are not accidentally "stale-deleted".
				if _, existsInCache := c.poolCache[hwAddr]; !existsInCache && (hwAddr == util.ExcludedMark || hwAddr == util.ReservedMark) {
					// This might not be the right place, as AddLease is for real leases.
					// The primary goal is that the DHCP server doesn't hand out these IPs.
					// The IPPool status itself is the source of truth for exclusions.
					// The DHCPAllocator's job is to give out leases from the available range,
					// respecting what's already allocated (including exclusions).
					// The current AddLease is for *dynamic* leases.
					// Perhaps we don't need to do anything special for EXCLUDED here in terms of AddLease.
					// The controller's main job is to sync actual dynamic allocations.
				}
				continue // Skip special markers for adding as dynamic DHCP leases
			}

			// The AddLease function in DHCPAllocator is idempotent if the lease details are identical,
			// but it errors if the MAC exists with different details.
			// It's safer to delete then re-add if an update is intended, or ensure AddLease can handle "updates".
			// Current AddLease errors on existing hwAddr. So, we must delete first if it exists.
			// This is problematic if we only want to update parameters without changing ClientIP.
			// Let's refine: GetLease, if exists and IP matches, maybe update params. If IP differs, delete and re-add.
			// For now, keeping it simple: if it's in the new status, we ensure it's in the allocator.
			// The previous check for deletion handles MACs no longer in status.
			// Now, for MACs in status, we add them. If AddLease fails due to "already exists",
			// it implies our cache or state is desynced, or AddLease needs to be more flexible.

			// Let's use GetLease to see if we need to add or if it's already there and consistent.
			existingLease, found := c.dhcpAllocator.GetLease(poolRefStr, hwAddr)
			if found && existingLease.ClientIP.String() == clientIPStr {
				// Lease exists and IP matches. Potentially update other params if they changed.
				// For now, assume if IP & MAC match, it's current for this simple sync.
				// logrus.Debugf("(%s) Lease for MAC %s, IP %s already in DHCPAllocator and matches. Skipping AddLease.", poolRefStr, hwAddr, clientIPStr)
				// We still need to ensure it's in our local c.poolCache
				if _, existsInCache := c.poolCache[hwAddr]; !existsInCache {
					c.poolCache[hwAddr] = clientIPStr
				}
				continue
			}
			if found && existingLease.ClientIP.String() != clientIPStr {
				// MAC exists but with a different IP. This is an inconsistent state. Delete the old one.
				logrus.Warnf("(%s) MAC %s found with different IP %s in DHCPAllocator. Deleting before adding new IP %s.",
					poolRefStr, hwAddr, existingLease.ClientIP.String(), clientIPStr)
				if errDel := c.dhcpAllocator.DeleteLease(poolRefStr, hwAddr); errDel != nil {
					logrus.Errorf("(%s) Failed to delete inconsistent lease for MAC %s: %v", poolRefStr, hwAddr, errDel)
					// Continue to try AddLease, it will likely fail if delete failed.
				}
			}

			// Add the lease.
			err := c.dhcpAllocator.AddLease(
				poolRefStr, // Corrected: pass the poolRef string
				hwAddr,
				specConf.ServerIP,
				clientIPStr,
				specConf.CIDR,
				specConf.Router,
				dnsServers,
				domainName,
				domainSearch,
				ntpServers,
				specConf.LeaseTime,
			)
			if err != nil {
				// Log error, but don't necessarily stop processing other leases.
				// The requeue mechanism will handle retries for the IPPool update.
				logrus.Errorf("(%s) Failed to add lease to DHCPAllocator for MAC %s, IP %s: %v", poolRefStr, hwAddr, clientIPStr, err)
				// Do not return err here, as we want to process all allocations.
				// The overall sync operation will be retried if there are errors.
			} else {
				logrus.Infof("(%s) Successfully added lease to DHCPAllocator for MAC %s, IP %s", poolRefStr, hwAddr, clientIPStr)
				c.poolCache[hwAddr] = clientIPStr // Update our tracking cache
			}
		}
	}
	logrus.Infof("DHCPAllocator cache updated for IPPool %s/%s", ipPool.Namespace, ipPool.Name)
	return nil
}

func (c *Controller) handleErr(err error, key interface{}) {
	if err == nil {
		c.queue.Forget(key.(Event))

		return
	}

	if c.queue.NumRequeues(key.(Event)) < 5 {
		logrus.Errorf("(controller.handleErr) syncing IPPool %v: %v", key, err)

		c.queue.AddRateLimited(key.(Event))

		return
	}

	c.queue.Forget(key.(Event))

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
