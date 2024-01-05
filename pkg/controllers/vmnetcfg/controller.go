package vmnetcfg

import (
	"context"
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/rancher/wrangler/pkg/kv"
	networkv1 "github.com/starbops/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/starbops/vm-dhcp-controller/pkg/config"
	ctlnetworkv1 "github.com/starbops/vm-dhcp-controller/pkg/generated/controllers/network.harvesterhci.io/v1alpha1"
	"github.com/starbops/vm-dhcp-controller/pkg/ipam"
)

const controllerName = "vm-dhcp-vmnetcfg-controller"

type Handler struct {
	IPAllocator *ipam.IPAllocator

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
		IPAllocator: management.IPAllocator,

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

	vmnetcfgs.OnRemove(ctx, controllerName, handler.OnRemove)

	return nil
}

func (h *Handler) Allocate(vmNetCfg *networkv1.VirtualMachineNetworkConfig, status networkv1.VirtualMachineNetworkConfigStatus) (networkv1.VirtualMachineNetworkConfigStatus, error) {
	klog.Infof("allocate ip for vmnetcfg %s/%s", vmNetCfg.Namespace, vmNetCfg.Name)

	ncStatuses := vmNetCfg.Status.NetworkConfig
	for _, nc := range vmNetCfg.Spec.NetworkConfig {
		if isStatusAllocated(ncStatuses, nc.MACAddress) {
			return status, nil
		}

		dIP := nc.IPAddress
		if dIP == nil {
			dIP = ipam.UnspecifiedIPAddress
		}

		ip, err := h.IPAllocator.AllocateIP(nc.NetworkName, dIP.String())
		if err != nil {
			return status, err
		}

		ncStatus := networkv1.NetworkConfigStatus{
			MACAddress:  nc.MACAddress,
			NetworkName: nc.NetworkName,
		}

		ipPoolNamespace, ipPoolName := kv.RSplit(nc.NetworkName, "/")
		ipPool, err := h.ippoolCache.Get(ipPoolNamespace, ipPoolName)
		if err != nil {
			return status, err
		}

		// Prepare to update IPPool status
		ipPoolCpy := ipPool.DeepCopy()

		ipv4Status := ipPoolCpy.Status.IPv4
		if ipv4Status == nil {
			ipv4Status = new(networkv1.IPv4Status)
		}

		allocated := ipv4Status.Allocated
		if allocated == nil {
			allocated = make(map[string]string)
		}

		allocated[ip.String()] = nc.MACAddress
		ncStatus.AllocatedIPAddress = ip
		ncStatus.Status = "Allocated"
		ncStatuses = append(ncStatuses, ncStatus)

		ipv4Status.Allocated = allocated
		ipPoolCpy.Status.IPv4 = ipv4Status

		if !reflect.DeepEqual(ipPoolCpy, ipPool) {
			klog.Infof("update ippool %s/%s", ipPool.Namespace, ipPool.Name)
			ipPoolCpy.Status.LastUpdate = metav1.Now()
			if _, err := h.ippoolClient.UpdateStatus(ipPoolCpy); err != nil {
				return status, err
			}
		}
	}

	status.NetworkConfig = ncStatuses

	return status, nil
}

func (h *Handler) OnRemove(key string, vmNetCfg *networkv1.VirtualMachineNetworkConfig) (*networkv1.VirtualMachineNetworkConfig, error) {
	if vmNetCfg == nil {
		return nil, nil
	}

	klog.Infof("vmnetcfg configuration %s/%s has been removed", vmNetCfg.Namespace, vmNetCfg.Name)

	for _, nc := range vmNetCfg.Status.NetworkConfig {
		// Deallocate IP address from IPAM
		isAllocated, err := h.IPAllocator.IsAllocated(nc.NetworkName, nc.AllocatedIPAddress.String())
		if err != nil {
			return vmNetCfg, err
		}
		if isAllocated {
			if err := h.IPAllocator.DeallocateIP(nc.NetworkName, nc.AllocatedIPAddress.String()); err != nil {
				return vmNetCfg, err
			}
		}

		ipPoolNamespace, ipPoolName := kv.RSplit(nc.NetworkName, "/")
		ipPool, err := h.ippoolCache.Get(ipPoolNamespace, ipPoolName)
		if err != nil {
			return vmNetCfg, err
		}

		ipPoolCpy := ipPool.DeepCopy()

		// Remove record in IPPool status
		delete(ipPoolCpy.Status.IPv4.Allocated, nc.AllocatedIPAddress.String())

		if !reflect.DeepEqual(ipPoolCpy, ipPool) {
			klog.Infof("update ippool %s/%s", ipPool.Namespace, ipPool.Name)
			ipPoolCpy.Status.LastUpdate = metav1.Now()
			if _, err := h.ippoolClient.UpdateStatus(ipPoolCpy); err != nil {
				return vmNetCfg, err
			}
		}
	}

	return vmNetCfg, nil
}

func isStatusAllocated(ncStatuses []networkv1.NetworkConfigStatus, mac string) bool {
	for _, ncStatus := range ncStatuses {
		if ncStatus.MACAddress == mac {
			return ncStatus.AllocatedIPAddress != nil
		}
	}
	return false
}
