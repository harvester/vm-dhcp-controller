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
	ctlnetworkv1 "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/network.harvesterhci.io/v1alpha1"
	"github.com/harvester/vm-dhcp-controller/pkg/ipam"
	"github.com/harvester/vm-dhcp-controller/pkg/metrics"
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
}

func Register(ctx context.Context, management *config.Management) error {
	vmnetcfgs := management.HarvesterNetworkFactory.Network().V1alpha1().VirtualMachineNetworkConfig()
	ippools := management.HarvesterNetworkFactory.Network().V1alpha1().IPPool()

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
	}

	ctlnetworkv1.RegisterVirtualMachineNetworkConfigStatusHandler(
		ctx,
		vmnetcfgs,
		networkv1.Allocated,
		"vmnetcfg-allocate",
		handler.Allocate,
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
		if err := h.cleanup(vmNetCfg); err != nil {
			return vmNetCfg, err
		}
		networkv1.Disabled.True(vmNetCfgCpy)
		updateAllNetworkConfigState(vmNetCfgCpy.Status.NetworkConfigs)
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

func (h *Handler) Allocate(vmNetCfg *networkv1.VirtualMachineNetworkConfig, status networkv1.VirtualMachineNetworkConfigStatus) (networkv1.VirtualMachineNetworkConfigStatus, error) {
	logrus.Debugf("(vmnetcfg.Allocate) allocate ip for vmnetcfg %s/%s", vmNetCfg.Namespace, vmNetCfg.Name)

	if vmNetCfg.Spec.Paused != nil && *vmNetCfg.Spec.Paused {
		return status, fmt.Errorf("vmnetcfg %s/%s was administratively disabled", vmNetCfg.Namespace, vmNetCfg.Name)
	}

	var ncStatuses []networkv1.NetworkConfigStatus
	for _, nc := range vmNetCfg.Spec.NetworkConfigs {
		exists, err := h.cacheAllocator.HasMAC(nc.NetworkName, nc.MACAddress)
		if err != nil {
			return status, err
		}
		if exists {
			// Recover IP from cache

			ip, err := h.cacheAllocator.GetIPByMAC(nc.NetworkName, nc.MACAddress)
			if err != nil {
				return status, err
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
			ipPoolNamespace, ipPoolName := kv.RSplit(nc.NetworkName, "/")
			if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				ipPool, err := h.ippoolCache.Get(ipPoolNamespace, ipPoolName)
				if err != nil {
					return err
				}

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
					_, err = h.ippoolClient.UpdateStatus(ipPoolCpy)
					return err
				}

				return nil
			}); err != nil {
				return status, err
			}

			continue
		}

		// Allocate new IP

		dIP := net.IPv4zero.String()
		if nc.IPAddress != nil {
			dIP = *nc.IPAddress
		}
		// Recover IP from status (resume from paused state)
		if oIP, err := findIPAddressFromNetworkConfigStatusByMACAddress(vmNetCfg.Status.NetworkConfigs, nc.MACAddress); err == nil {
			dIP = oIP
		}

		ip, err := h.ipAllocator.AllocateIP(nc.NetworkName, dIP)
		if err != nil {
			return status, err
		}

		if err := h.cacheAllocator.AddMAC(nc.NetworkName, nc.MACAddress, ip); err != nil {
			return status, err
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
		ipPoolNamespace, ipPoolName := kv.RSplit(nc.NetworkName, "/")
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			ipPool, err := h.ippoolCache.Get(ipPoolNamespace, ipPoolName)
			if err != nil {
				return err
			}

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
				_, err = h.ippoolClient.UpdateStatus(ipPoolCpy)
				return err
			}

			return nil
		}); err != nil {
			return status, err
		}
	}

	status.NetworkConfigs = ncStatuses

	return status, nil
}

func (h *Handler) OnRemove(key string, vmNetCfg *networkv1.VirtualMachineNetworkConfig) (*networkv1.VirtualMachineNetworkConfig, error) {
	if vmNetCfg == nil {
		return nil, nil
	}

	logrus.Debugf("(vmnetcfg.OnRemove) vmnetcfg configuration %s/%s has been removed", vmNetCfg.Namespace, vmNetCfg.Name)

	if err := h.cleanup(vmNetCfg); err != nil {
		return vmNetCfg, err
	}

	return vmNetCfg, nil
}

func (h *Handler) cleanup(vmNetCfg *networkv1.VirtualMachineNetworkConfig) error {
	h.metricsAllocator.DeleteVmNetCfgStatus(vmNetCfg.Namespace + "/" + vmNetCfg.Name)

	for _, ncStatus := range vmNetCfg.Status.NetworkConfigs {
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

		ipPoolNamespace, ipPoolName := kv.RSplit(ncStatus.NetworkName, "/")
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			ipPool, err := h.ippoolCache.Get(ipPoolNamespace, ipPoolName)
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

func updateAllNetworkConfigState(ncStatuses []networkv1.NetworkConfigStatus) {
	for i := range ncStatuses {
		ncStatuses[i].State = networkv1.PendingState
	}
}
