package vmnetcfg

import (
	"context"
	"fmt"
	"net"
	"reflect"

	"github.com/rancher/wrangler/pkg/kv"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	networkv1 "github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/harvester/vm-dhcp-controller/pkg/cache"
	"github.com/harvester/vm-dhcp-controller/pkg/config"
	ctlcniv1 "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/k8s.cni.cncf.io/v1"
	ctlnetworkv1 "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/network.harvesterhci.io/v1alpha1"
	"github.com/harvester/vm-dhcp-controller/pkg/ipam"
	"github.com/harvester/vm-dhcp-controller/pkg/metrics"
	"github.com/harvester/vm-dhcp-controller/pkg/util"
)

const controllerName = "vm-dhcp-vmnetcfg-controller"

type Handler struct {
	cacheAllocator   *cache.CacheAllocator
	ipAllocator      *ipam.IPAllocator
	metricsAllocator *metrics.MetricsAllocator

	vmnetcfgController ctlnetworkv1.VirtualMachineNetworkConfigController
	vmnetcfgClient     ctlnetworkv1.VirtualMachineNetworkConfigClient
	vmnetcfgCache      ctlnetworkv1.VirtualMachineNetworkConfigCache
	ippoolController   ctlnetworkv1.IPPoolController
	ippoolClient       ctlnetworkv1.IPPoolClient
	ippoolCache        ctlnetworkv1.IPPoolCache
	nadCache           ctlcniv1.NetworkAttachmentDefinitionCache
}

func Register(ctx context.Context, management *config.Management) error {
	vmnetcfgs := management.HarvesterNetworkFactory.Network().V1alpha1().VirtualMachineNetworkConfig()
	ippools := management.HarvesterNetworkFactory.Network().V1alpha1().IPPool()
	nads := management.CniFactory.K8s().V1().NetworkAttachmentDefinition()

	handler := &Handler{
		cacheAllocator:   management.CacheAllocator,
		ipAllocator:      management.IPAllocator,
		metricsAllocator: management.MetricsAllocator,

		vmnetcfgController: vmnetcfgs,
		vmnetcfgClient:     vmnetcfgs,
		vmnetcfgCache:      vmnetcfgs.Cache(),
		ippoolController:   ippools,
		ippoolClient:       ippools,
		ippoolCache:        ippools.Cache(),
		nadCache:           nads.Cache(),
	}

	ctlnetworkv1.RegisterVirtualMachineNetworkConfigStatusHandler(
		ctx,
		vmnetcfgs,
		networkv1.Allocated,
		"vmnetcfg-allocate",
		handler.Allocate,
	)

	ctlnetworkv1.RegisterVirtualMachineNetworkConfigStatusHandler(
		ctx,
		vmnetcfgs,
		networkv1.InSynced,
		"vmnetcfg-sync",
		handler.Sync,
	)

	vmnetcfgs.OnChange(ctx, controllerName, handler.OnChange)
	vmnetcfgs.OnRemove(ctx, controllerName, handler.OnRemove)

	return nil
}

func (h *Handler) OnChange(key string, vmNetCfg *networkv1.VirtualMachineNetworkConfig) (*networkv1.VirtualMachineNetworkConfig, error) {
	if vmNetCfg == nil || vmNetCfg.DeletionTimestamp != nil {
		return nil, nil
	}

	logrus.Debugf("(vmnetcfg.OnChange) vmnetcfg configuration %s has been changed: %+v", key, vmNetCfg.Spec.NetworkConfigs)

	vmNetCfgCpy := vmNetCfg.DeepCopy()

	// Check if the VirtualMachineNetworkConfig is administratively disabled
	if vmNetCfg.Spec.Paused != nil && *vmNetCfg.Spec.Paused {
		logrus.Infof("(vmnetcfg.OnChange) try to cleanup ipam and cache, and update ippool status for vmnetcfg %s", key)
		if err := h.cleanup(vmNetCfg, false); err != nil {
			return vmNetCfg, err
		}
		networkv1.Disabled.True(vmNetCfgCpy)
		updateAllNetworkConfigState(vmNetCfgCpy.Status.NetworkConfigs, networkv1.PendingState)
		if !reflect.DeepEqual(vmNetCfgCpy, vmNetCfg) {
			return h.vmnetcfgClient.UpdateStatus(vmNetCfgCpy)
		}
		return vmNetCfg, nil
	}
	networkv1.Disabled.False(vmNetCfgCpy)

	if !reflect.DeepEqual(vmNetCfgCpy, vmNetCfg) {
		return h.vmnetcfgClient.UpdateStatus(vmNetCfgCpy)
	}

	return vmNetCfg, nil
}

// Allocate allocates IP addresses for the VirtualMachineNetworkConfig only
// when it is in-synced.
func (h *Handler) Allocate(vmNetCfg *networkv1.VirtualMachineNetworkConfig, status networkv1.VirtualMachineNetworkConfigStatus) (networkv1.VirtualMachineNetworkConfigStatus, error) {
	logrus.Debugf("(vmnetcfg.Allocate) allocate ip for vmnetcfg %s/%s", vmNetCfg.Namespace, vmNetCfg.Name)

	if vmNetCfg.Spec.Paused != nil && *vmNetCfg.Spec.Paused {
		return status, fmt.Errorf("vmnetcfg %s/%s was administratively disabled", vmNetCfg.Namespace, vmNetCfg.Name)
	}

	// Only start the allocation process if the VirtualMachineNetworkConfig is in-synced
	if networkv1.InSynced.IsFalse(vmNetCfg) {
		return status, fmt.Errorf("vmnetcfg %s/%s is out-of-sync; waiting for reconcile", vmNetCfg.Namespace, vmNetCfg.Name)
	}

	var ncStatuses []networkv1.NetworkConfigStatus
	for _, nc := range vmNetCfg.Spec.NetworkConfigs {
		ipPool, err := h.getIPPoolFromNetworkConfig(nc)
		if err != nil {
			return status, err
		}
		if !networkv1.CacheReady.IsTrue(ipPool) {
			return status, fmt.Errorf("ippool %s/%s is not ready", ipPool.Namespace, ipPool.Name)
		}

		exists, err := h.cacheAllocator.HasMAC(nc.NetworkName, nc.MACAddress)
		if err != nil {
			return status, err
		}

		var ip string

		if exists {
			// Recover IP from cache
			ip, err = h.cacheAllocator.GetIPByMAC(nc.NetworkName, nc.MACAddress)
			if err != nil {
				return status, err
			}
		} else {
			dIP := net.IPv4zero.String()
			if nc.IPAddress != nil {
				dIP = *nc.IPAddress
			}

			// Recover IP from status (resume from paused state)
			if oIP, err := findIPAddressFromNetworkConfigStatusByMACAddress(vmNetCfg.Status.NetworkConfigs, nc.MACAddress); err == nil {
				dIP = oIP
			}

			// Allocate new IP
			ip, err = h.ipAllocator.AllocateIP(nc.NetworkName, dIP)
			if err != nil {
				return status, err
			}

			if err := h.cacheAllocator.AddMAC(nc.NetworkName, nc.MACAddress, ip); err != nil {
				return status, err
			}
		}

		// Prepare VirtualMachineNetworkConfig status
		ncStatus := networkv1.NetworkConfigStatus{
			AllocatedIPAddress: ip,
			MACAddress:         nc.MACAddress,
			NetworkName:        nc.NetworkName,
			State:              networkv1.AllocatedState,
		}

		ncStatuses = append(ncStatuses, ncStatus)

		// Update VirtualMachineNetworkConfig metrics
		h.metricsAllocator.UpdateVmNetCfgStatus(
			fmt.Sprintf("%s/%s", vmNetCfg.Namespace, vmNetCfg.Name),
			ncStatus.NetworkName,
			ncStatus.MACAddress,
			ncStatus.AllocatedIPAddress,
			string(ncStatus.State),
		)

		// Update IPPool status
		ipPoolCpy := ipPool.DeepCopy()

		ipv4Status := ipPoolCpy.Status.IPv4
		if ipv4Status == nil {
			ipv4Status = new(networkv1.IPv4Status)
		}

		allocated := ipv4Status.Allocated
		if allocated == nil {
			allocated = make(map[string]string)
		}

		allocated[ip] = nc.MACAddress

		ipv4Status.Allocated = allocated
		ipPoolCpy.Status.IPv4 = ipv4Status

		if !reflect.DeepEqual(ipPoolCpy, ipPool) {
			logrus.Infof("(vmnetcfg.Allocate) update ippool %s/%s", ipPool.Namespace, ipPool.Name)
			ipPoolCpy.Status.LastUpdate = metav1.Now()
			if _, err = h.ippoolClient.UpdateStatus(ipPoolCpy); err != nil {
				return status, err
			}
		}
	}

	status.NetworkConfigs = ncStatuses

	return status, nil
}

// Sync ensures that the VirtualMachineNetworkConfig is in-sync by
// comparing the Spec and Status and cleaning up stale records.
func (h *Handler) Sync(vmNetCfg *networkv1.VirtualMachineNetworkConfig, status networkv1.VirtualMachineNetworkConfigStatus) (networkv1.VirtualMachineNetworkConfigStatus, error) {
	logrus.Debugf("(vmnetcfg.InSynced) syncing vmnetcfg %s/%s", vmNetCfg.Namespace, vmNetCfg.Name)

	if vmNetCfg.Spec.Paused != nil && *vmNetCfg.Spec.Paused {
		return status, fmt.Errorf("vmnetcfg %s/%s was administratively disabled", vmNetCfg.Namespace, vmNetCfg.Name)
	}

	if len(vmNetCfg.Spec.NetworkConfigs) == 0 {
		return status, fmt.Errorf("vmnetcfg %s/%s has no network configs", vmNetCfg.Namespace, vmNetCfg.Name)
	}

	// Nothing to do if the VirtualMachineNetworkConfig is already in-sync
	if networkv1.InSynced.IsTrue(vmNetCfg) {
		logrus.Debugf("(vmnetcfg.InSynced) vmnetcfg %s/%s is in-sync", vmNetCfg.Namespace, vmNetCfg.Name)
		return status, nil
	}

	logrus.Infof("(vmnetcfg.InSynced) vmnetcfg %s/%s is out-of-sync; start reconciling", vmNetCfg.Namespace, vmNetCfg.Name)

	// Build a set of MAC addresses from the Spec
	var macAddressSet = make(map[string]struct{})
	for _, nc := range vmNetCfg.Spec.NetworkConfigs {
		macAddressSet[nc.MACAddress] = struct{}{}
	}

	// Mark the NetworkConfigStatus as stale if the MAC address is not in the Spec
	for i, ncStatus := range status.NetworkConfigs {
		if _, ok := macAddressSet[ncStatus.MACAddress]; !ok {
			status.NetworkConfigs[i].State = networkv1.StaleState
		}
	}

	// Cleanup the stale records
	if err := h.cleanup(vmNetCfg, true); err != nil {
		return status, err
	}

	// Remove the stale NetworkConfigStatus from the status
	var nonStaleNetworkConfigs []networkv1.NetworkConfigStatus
	for _, ncStatus := range status.NetworkConfigs {
		if ncStatus.State != networkv1.StaleState {
			nonStaleNetworkConfigs = append(nonStaleNetworkConfigs, ncStatus)
		}
	}
	status.NetworkConfigs = nonStaleNetworkConfigs

	return status, nil
}

func (h *Handler) OnRemove(key string, vmNetCfg *networkv1.VirtualMachineNetworkConfig) (*networkv1.VirtualMachineNetworkConfig, error) {
	if vmNetCfg == nil {
		return nil, nil
	}

	logrus.Debugf("(vmnetcfg.OnRemove) vmnetcfg configuration %s/%s has been removed", vmNetCfg.Namespace, vmNetCfg.Name)

	if err := h.cleanup(vmNetCfg, false); err != nil {
		return vmNetCfg, err
	}

	return vmNetCfg, nil
}

func (h *Handler) cleanup(vmNetCfg *networkv1.VirtualMachineNetworkConfig, cleanupStaleOnly bool) error {
	if !cleanupStaleOnly {
		h.metricsAllocator.DeleteVmNetCfgStatus(vmNetCfg.Namespace + "/" + vmNetCfg.Name)
	}

	for _, ncStatus := range vmNetCfg.Status.NetworkConfigs {
		if !cleanupStaleOnly || ncStatus.State == networkv1.StaleState {
			// Deallocate IP address from IPAM
			isAllocated, err := h.ipAllocator.IsAllocated(ncStatus.NetworkName, ncStatus.AllocatedIPAddress)
			if err != nil {
				return err
			}
			if isAllocated {
				if err := h.ipAllocator.DeallocateIP(ncStatus.NetworkName, ncStatus.AllocatedIPAddress); err != nil {
					return err
				}
			}

			// Remove entry from cache
			exists, err := h.cacheAllocator.HasMAC(ncStatus.NetworkName, ncStatus.MACAddress)
			if err != nil {
				return err
			}
			if exists {
				if err := h.cacheAllocator.DeleteMAC(ncStatus.NetworkName, ncStatus.MACAddress); err != nil {
					return err
				}
			}

			if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				ipPool, err := h.getIPPoolFromNetworkConfigStatus(ncStatus)
				if err != nil {
					return err
				}

				ipPoolCpy := ipPool.DeepCopy()

				// Remove record in IPPool status
				delete(ipPoolCpy.Status.IPv4.Allocated, ncStatus.AllocatedIPAddress)

				if !reflect.DeepEqual(ipPoolCpy, ipPool) {
					logrus.Infof("(vmnetcfg.cleanup) update ippool %s/%s", ipPool.Namespace, ipPool.Name)
					ipPoolCpy.Status.LastUpdate = metav1.Now()
					_, err := h.ippoolClient.UpdateStatus(ipPoolCpy)
					return err
				}

				return nil
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func findIPAddressFromNetworkConfigStatusByMACAddress(ncStatuses []networkv1.NetworkConfigStatus, macAddress string) (ipAddress string, err error) {
	for _, ncStatus := range ncStatuses {
		if ncStatus.MACAddress == macAddress && ncStatus.AllocatedIPAddress != "" {
			return ncStatus.AllocatedIPAddress, nil
		}
	}
	return net.IPv4zero.String(), fmt.Errorf("could not find allocated ip for mac %s", macAddress)
}

func (h *Handler) getIPPoolFromNetworkName(networkName string) (*networkv1.IPPool, error) {
	nadNamespace, nadName := kv.RSplit(networkName, "/")
	nad, err := h.nadCache.Get(nadNamespace, nadName)
	if err != nil {
		return nil, err
	}
	if nad.Labels == nil {
		return nil, fmt.Errorf("network attachment definition %s/%s has no labels", nadNamespace, nadName)
	}
	ipPoolNamespace, ok := nad.Labels[util.IPPoolNamespaceLabelKey]
	if !ok {
		return nil, fmt.Errorf("network attachment definition %s/%s has no label %s", nadNamespace, nadName, util.IPPoolNamespaceLabelKey)
	}
	ipPoolName, ok := nad.Labels[util.IPPoolNameLabelKey]
	if !ok {
		return nil, fmt.Errorf("network attachment definition %s/%s has no label %s", nadNamespace, nadName, util.IPPoolNameLabelKey)
	}
	return h.ippoolCache.Get(ipPoolNamespace, ipPoolName)
}

func (h *Handler) getIPPoolFromNetworkConfig(nc networkv1.NetworkConfig) (*networkv1.IPPool, error) {
	return h.getIPPoolFromNetworkName(nc.NetworkName)
}

func (h *Handler) getIPPoolFromNetworkConfigStatus(ncStatus networkv1.NetworkConfigStatus) (*networkv1.IPPool, error) {
	return h.getIPPoolFromNetworkName(ncStatus.NetworkName)
}
