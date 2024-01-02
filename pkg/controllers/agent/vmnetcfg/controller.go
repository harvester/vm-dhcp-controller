package vmnetcfg

import (
	"context"
	"fmt"
	"net"
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	"github.com/rancher/wrangler/pkg/relatedresource"
	networkv1 "github.com/starbops/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/starbops/vm-dhcp-controller/pkg/config"
	"github.com/starbops/vm-dhcp-controller/pkg/dhcp"
	ctlnetworkv1 "github.com/starbops/vm-dhcp-controller/pkg/generated/controllers/network.harvesterhci.io/v1alpha1"
	"github.com/starbops/vm-dhcp-controller/pkg/ipam"
)

const controllerName = "vm-dhcp-vmnetcfg-controller"

type Handler struct {
	poolRef types.NamespacedName

	dhcpAllocator *dhcp.DHCPAllocator
	IPAllocator   *ipam.IPAllocator

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
		poolRef: management.Options.PoolRef,

		dhcpAllocator: management.DHCPAllocator,
		IPAllocator:   management.IPAllocator,

		vmnetcfgController: vmnetcfgs,
		vmnetcfgClient:     vmnetcfgs,
		vmnetcfgCache:      vmnetcfgs.Cache(),
		ippoolController:   ippools,
		ippoolClient:       ippools,
		ippoolCache:        ippools.Cache(),
	}

	vmnetcfgs.OnChange(ctx, controllerName, handler.OnChange)
	vmnetcfgs.OnRemove(ctx, controllerName, handler.OnRemove)
	relatedresource.Watch(ctx, "vmnetcfg-trigger", func(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
		var keys []relatedresource.Key
		vmNetCfgs, err := handler.vmnetcfgCache.List(namespace, labels.Everything())
		if err != nil {
			return nil, err
		}
		for _, vmNetCfg := range vmNetCfgs {
			key := relatedresource.Key{
				Namespace: namespace,
				Name:      vmNetCfg.Name,
			}
			keys = append(keys, key)
		}
		return keys, nil
	}, vmnetcfgs, ippools)

	return nil
}

func (h *Handler) OnChange(key string, vmNetCfg *networkv1.VirtualMachineNetworkConfig) (*networkv1.VirtualMachineNetworkConfig, error) {
	if vmNetCfg == nil || vmNetCfg.DeletionTimestamp != nil {
		return nil, nil
	}

	klog.Infof("vmnetcfg configuration %s/%s has been changed: %+v", vmNetCfg.Namespace, vmNetCfg.Name, vmNetCfg.Spec.NetworkConfig)

	ipPool, err := h.ippoolCache.Get(h.poolRef.Namespace, h.poolRef.Name)
	if err != nil {
		return vmNetCfg, err
	}

	ref := fmt.Sprintf("%s/%s", vmNetCfg.Namespace, vmNetCfg.Spec.VMName)

	// Prepare to update IPPool status
	ipPoolCpy := ipPool.DeepCopy()

	// ipv4Status := util.ExistedOrNew(ipPoolCpy.Status.IPv4, new(networkv1.IPv4Status)).(*networkv1.IPv4Status)
	ipv4Status := ipPoolCpy.Status.IPv4
	if ipv4Status == nil {
		ipv4Status = new(networkv1.IPv4Status)
	}
	// allocated := util.ExistedOrNew(ipv4Status.Allocated, map[string]string{}).(map[string]string)
	allocated := ipv4Status.Allocated
	if allocated == nil {
		allocated = make(map[string]string)
	}

	networkConfigStatuses := vmNetCfg.Status.NetworkConfig

	for _, nc := range vmNetCfg.Spec.NetworkConfig {
		ncs := networkv1.NetworkConfigStatus{
			MACAddress:  nc.MACAddress,
			NetworkName: nc.NetworkName,
		}

		if h.dhcpAllocator.CheckLease(nc.MACAddress) {
			klog.Infof("dhcp lease for mac address %s existed", nc.MACAddress)
			continue
		}

		// Allocate IP address from IPAM
		var ip net.IP
		if nc.IPAddress == nil {
			ip, err = h.IPAllocator.AllocateIP(nc.NetworkName, ipam.UnspecifiedIPAddress.String())
			if err != nil {
				return vmNetCfg, err
			}
		} else {
			ip, err = h.IPAllocator.AllocateIP(nc.NetworkName, nc.IPAddress.String())
			if err != nil {
				return vmNetCfg, err
			}
		}

		// Add lease with the allocated IP address
		if err := h.dhcpAllocator.AddLease(
			nc.MACAddress,
			ipPool.Spec.IPv4Config.ServerIP,
			ip,
			ipPool.Spec.IPv4Config.CIDR,
			ipPool.Spec.IPv4Config.Router,
			ipPool.Spec.IPv4Config.DNS,
			*ipPool.Spec.IPv4Config.DomainName,
			ipPool.Spec.IPv4Config.DomainSearch,
			ipPool.Spec.IPv4Config.NTP,
			*ipPool.Spec.IPv4Config.LeaseTime,
			ref,
		); err != nil {
			return vmNetCfg, err
		}

		allocated[ip.String()] = nc.MACAddress
		ncs.AllocatedIPAddress = ip
		networkConfigStatuses = append(networkConfigStatuses, ncs)
	}

	ipv4Status.Allocated = allocated
	ipPoolCpy.Status.IPv4 = ipv4Status

	if !reflect.DeepEqual(ipPoolCpy, ipPool) {
		klog.Infof("update ippool %s/%s", ipPool.Namespace, ipPool.Name)
		ipPoolCpy.Status.LastUpdate = metav1.Now()
		if _, err := h.ippoolClient.UpdateStatus(ipPoolCpy); err != nil {
			return vmNetCfg, err
		}
	}

	vmNetCfgCpy := vmNetCfg.DeepCopy()
	vmNetCfgCpy.Status.NetworkConfig = networkConfigStatuses

	if !reflect.DeepEqual(vmNetCfgCpy, vmNetCfg) {
		klog.Infof("update vmnetcfg %s/%s", vmNetCfg.Namespace, vmNetCfg.Name)
		return h.vmnetcfgClient.UpdateStatus(vmNetCfgCpy)
	}

	return vmNetCfg, nil
}

func (h *Handler) OnRemove(key string, vmNetCfg *networkv1.VirtualMachineNetworkConfig) (*networkv1.VirtualMachineNetworkConfig, error) {
	if vmNetCfg == nil {
		return nil, nil
	}

	klog.Infof("vmnetcfg configuration %s/%s has been removed", vmNetCfg.Namespace, vmNetCfg.Name)

	ipPool, err := h.ippoolCache.Get(h.poolRef.Namespace, h.poolRef.Name)
	if err != nil {
		return vmNetCfg, err
	}

	ipPoolCpy := ipPool.DeepCopy()

	for _, nc := range vmNetCfg.Spec.NetworkConfig {
		var lease dhcp.DHCPLease

		// Delete DHCP lease
		if h.dhcpAllocator.CheckLease(nc.MACAddress) {
			lease = h.dhcpAllocator.GetLease(nc.MACAddress)
			if err := h.dhcpAllocator.DeleteLease(nc.MACAddress); err != nil {
				return vmNetCfg, err
			}
		}

		// Deallocate IP address from IPAM
		isAllocated, err := h.IPAllocator.IsAllocated(nc.NetworkName, lease.ClientIP.String())
		if err != nil {
			return vmNetCfg, err
		}
		if isAllocated {
			if err := h.IPAllocator.DeallocateIP(nc.NetworkName, lease.ClientIP.String()); err != nil {
				return vmNetCfg, err
			}
		}

		// Remove record in IPPool status
		delete(ipPoolCpy.Status.IPv4.Allocated, lease.ClientIP.String())
	}

	if !reflect.DeepEqual(ipPoolCpy, ipPool) {
		klog.Infof("update ippool %s/%s", ipPool.Namespace, ipPool.Name)
		ipPoolCpy.Status.LastUpdate = metav1.Now()
		if _, err := h.ippoolClient.UpdateStatus(ipPoolCpy); err != nil {
			return vmNetCfg, err
		}
	}

	return vmNetCfg, nil
}
