package vmnetcfg

import (
	"context"
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
)

const controllerName = "vm-dhcp-vmnetcfg-controller"

type Handler struct {
	cacheAllocator *cache.CacheAllocator
	ipAllocator    *ipam.IPAllocator

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
		cacheAllocator: management.CacheAllocator,
		ipAllocator:    management.IPAllocator,

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
	logrus.Debugf("allocate ip for vmnetcfg %s/%s", vmNetCfg.Namespace, vmNetCfg.Name)

	ncStatuses := vmNetCfg.Status.NetworkConfig
	for _, nc := range vmNetCfg.Spec.NetworkConfig {
		// Check cache
		exists, err := h.cacheAllocator.HasEntry(nc.NetworkName, nc.MACAddress)
		if err != nil {
			return status, err
		}
		if exists {
			continue
		}

		dIP := nc.IPAddress
		if dIP == nil {
			dIP = ipam.UnspecifiedIPAddress
		}

		ip, err := h.ipAllocator.AllocateIP(nc.NetworkName, dIP.String())
		if err != nil {
			return status, err
		}

		// Update cache
		if err := h.cacheAllocator.AddEntry(nc.NetworkName, nc.MACAddress); err != nil {
			return status, err
		}

		ncStatus := networkv1.NetworkConfigStatus{
			AllocatedIPAddress: ip,
			MACAddress:         nc.MACAddress,
			NetworkName:        nc.NetworkName,
			Status:             "Allocated",
		}
		ncStatuses = append(ncStatuses, ncStatus)
		status.NetworkConfig = ncStatuses

		// Prepare to update IPPool status
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			ipPoolNamespace, ipPoolName := kv.RSplit(nc.NetworkName, "/")
			ipPool, err := h.ippoolClient.Get(ipPoolNamespace, ipPoolName, metav1.GetOptions{})
			if err != nil {
				return err
			}

			ipv4Status := ipPool.Status.IPv4
			if ipv4Status == nil {
				ipv4Status = new(networkv1.IPv4Status)
			}

			allocated := ipv4Status.Allocated
			if allocated == nil {
				allocated = make(map[string]string)
			}

			allocated[ip.String()] = nc.MACAddress

			ipv4Status.Allocated = allocated
			ipPool.Status.IPv4 = ipv4Status

			logrus.Infof("update ippool %s/%s", ipPool.Namespace, ipPool.Name)
			ipPool.Status.LastUpdate = metav1.Now()
			_, err = h.ippoolClient.UpdateStatus(ipPool)
			return err
		}); err != nil {
			return status, err
		}

		return status, nil
	}

	return status, nil
}

func (h *Handler) OnRemove(key string, vmNetCfg *networkv1.VirtualMachineNetworkConfig) (*networkv1.VirtualMachineNetworkConfig, error) {
	if vmNetCfg == nil {
		return nil, nil
	}

	logrus.Debugf("vmnetcfg configuration %s/%s has been removed", vmNetCfg.Namespace, vmNetCfg.Name)

	for _, nc := range vmNetCfg.Status.NetworkConfig {
		// Deallocate IP address from IPAM
		isAllocated, err := h.ipAllocator.IsAllocated(nc.NetworkName, nc.AllocatedIPAddress.String())
		if err != nil {
			return vmNetCfg, err
		}
		if isAllocated {
			if err := h.ipAllocator.DeallocateIP(nc.NetworkName, nc.AllocatedIPAddress.String()); err != nil {
				return vmNetCfg, err
			}
		}

		// Remove entry from cache
		exists, err := h.cacheAllocator.HasEntry(nc.NetworkName, nc.MACAddress)
		if err != nil {
			return vmNetCfg, err
		}
		if exists {
			if err := h.cacheAllocator.DeleteEntry(nc.NetworkName, nc.MACAddress); err != nil {
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
			logrus.Infof("update ippool %s/%s", ipPool.Namespace, ipPool.Name)
			ipPoolCpy.Status.LastUpdate = metav1.Now()
			if _, err := h.ippoolClient.UpdateStatus(ipPoolCpy); err != nil {
				return vmNetCfg, err
			}
		}
	}

	return vmNetCfg, nil
}
