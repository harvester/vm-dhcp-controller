package vmnetcfg

import (
	"context"
	"fmt"
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

	vmnetcfgs.OnRemove(ctx, controllerName, handler.OnRemove)

	return nil
}

func (h *Handler) Allocate(vmNetCfg *networkv1.VirtualMachineNetworkConfig, status networkv1.VirtualMachineNetworkConfigStatus) (networkv1.VirtualMachineNetworkConfigStatus, error) {
	logrus.Debugf("(vmnetcfg.Allocate) allocate ip for vmnetcfg %s/%s", vmNetCfg.Namespace, vmNetCfg.Name)

	var ncStatuses []networkv1.NetworkConfigStatus
	for _, nc := range vmNetCfg.Spec.NetworkConfig {
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
				Status:             "Allocated",
			}
			ncStatuses = append(ncStatuses, ncStatus)

			// Update VirtualMachineNetworkConfig metrics
			h.metricsAllocator.UpdateVmNetCfgStatus(
				fmt.Sprintf("%s/%s", vmNetCfg.Namespace, vmNetCfg.Name),
				ncStatus.NetworkName,
				ncStatus.MACAddress,
				ncStatus.AllocatedIPAddress,
				ncStatus.Status,
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
		dIP := util.UnspecifiedIPAddress
		if nc.IPAddress != nil {
			dIP = *nc.IPAddress
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
			Status:             "Allocated",
		}
		ncStatuses = append(ncStatuses, ncStatus)

		// Update VirtualMachineNetworkConfig metrics
		h.metricsAllocator.UpdateVmNetCfgStatus(
			fmt.Sprintf("%s/%s", vmNetCfg.Namespace, vmNetCfg.Name),
			ncStatus.NetworkName,
			ncStatus.MACAddress,
			ncStatus.AllocatedIPAddress,
			ncStatus.Status,
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

	status.NetworkConfig = ncStatuses

	return status, nil
}

func (h *Handler) OnRemove(key string, vmNetCfg *networkv1.VirtualMachineNetworkConfig) (*networkv1.VirtualMachineNetworkConfig, error) {
	if vmNetCfg == nil {
		return nil, nil
	}

	logrus.Debugf("(vmnetcfg.OnRemove) vmnetcfg configuration %s/%s has been removed", vmNetCfg.Namespace, vmNetCfg.Name)

	h.metricsAllocator.DeleteVmNetCfgStatus(key)

	for _, nc := range vmNetCfg.Status.NetworkConfig {
		// Deallocate IP address from IPAM
		isAllocated, err := h.ipAllocator.IsAllocated(nc.NetworkName, nc.AllocatedIPAddress)
		if err != nil {
			return vmNetCfg, err
		}
		if isAllocated {
			if err := h.ipAllocator.DeallocateIP(nc.NetworkName, nc.AllocatedIPAddress); err != nil {
				return vmNetCfg, err
			}
		}

		// Remove entry from cache
		exists, err := h.cacheAllocator.HasMAC(nc.NetworkName, nc.MACAddress)
		if err != nil {
			return vmNetCfg, err
		}
		if exists {
			if err := h.cacheAllocator.DeleteMAC(nc.NetworkName, nc.MACAddress); err != nil {
				return vmNetCfg, err
			}
		}

		ipPoolNamespace, ipPoolName := kv.RSplit(nc.NetworkName, "/")
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			ipPool, err := h.ippoolCache.Get(ipPoolNamespace, ipPoolName)
			if err != nil {
				return err
			}

			ipPoolCpy := ipPool.DeepCopy()

			// Remove record in IPPool status
			delete(ipPoolCpy.Status.IPv4.Allocated, nc.AllocatedIPAddress)

			if !reflect.DeepEqual(ipPoolCpy, ipPool) {
				logrus.Infof("(vmnetcfg.OnRemove) update ippool %s/%s", ipPool.Namespace, ipPool.Name)
				ipPoolCpy.Status.LastUpdate = metav1.Now()
				_, err := h.ippoolClient.UpdateStatus(ipPoolCpy)
				return err
			}

			return nil
		}); err != nil {
			return vmNetCfg, err
		}
	}

	return vmNetCfg, nil
}
