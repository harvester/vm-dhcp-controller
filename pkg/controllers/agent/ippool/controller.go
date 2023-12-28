package ippool

import (
	"context"
	"os"
	"reflect"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	networkv1 "github.com/starbops/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/starbops/vm-dhcp-controller/pkg/config"
	"github.com/starbops/vm-dhcp-controller/pkg/dhcp"
	ctlnetworkv1 "github.com/starbops/vm-dhcp-controller/pkg/generated/controllers/network.harvesterhci.io/v1alpha1"
	"github.com/starbops/vm-dhcp-controller/pkg/ipam"
)

const (
	controllerName = "vm-dhcp-ippool-controller"

	nic          = "eth1"
	PIDFilePath  = "/tmp/dhcpd.pid"
	excludedMark = "EXCLUDED"
)

type Handler struct {
	ctx context.Context

	dryRun  bool
	poolRef types.NamespacedName

	dhcpAllocator *dhcp.DHCPAllocator
	ipAllocator   *ipam.IPAllocator

	ippoolClient ctlnetworkv1.IPPoolClient
	ippoolCache  ctlnetworkv1.IPPoolCache
}

func Register(ctx context.Context, management *config.Management) error {
	ippools := management.HarvesterNetworkFactory.Network().V1alpha1().IPPool()

	handler := &Handler{
		ctx: ctx,

		dryRun:  management.Options.DryRun,
		poolRef: management.Options.PoolRef,

		dhcpAllocator: management.DHCPAllocator,
		ipAllocator:   management.IPAllocator,

		ippoolClient: ippools,
		ippoolCache:  ippools.Cache(),
	}

	ippools.OnChange(ctx, controllerName, handler.OnChange)

	return nil
}

func (h *Handler) OnChange(key string, ipPool *networkv1.IPPool) (*networkv1.IPPool, error) {
	if ipPool == nil || ipPool.DeletionTimestamp != nil {
		return nil, nil
	}

	ipPoolNamespacedName := types.NamespacedName{
		Namespace: ipPool.Namespace,
		Name:      ipPool.Name,
	}

	// The agent only focuses on the designated IPPool object
	if ipPoolNamespacedName != h.poolRef {
		return ipPool, nil
	}

	klog.Infof("ippool configuration %s has been changed: %+v", ipPoolNamespacedName, ipPool.Spec.IPv4Config)

	// Run the embedded DHCP server
	if h.dryRun {
		klog.Info("dry-run mode")
	} else {
		if _, err := os.Stat(PIDFilePath); err != nil {
			klog.Infof("start the embedded dhcp server for ippool %s/%s", ipPool.Namespace, ipPool.Name)
			if err := h.dhcpAllocator.Run(h.ctx, nic); err != nil {
				return ipPool, err
			}

			// Touch pid file for the readiness probe
			pid := os.Getpid()
			if err := os.WriteFile(PIDFilePath, []byte(strconv.Itoa(pid)), 0644); err != nil {
				return ipPool, err
			}
		}
	}

	klog.Infof("embedded dhcp server for ippool %s/%s is ready", ipPool.Namespace, ipPool.Name)

	// Construct the IPAM module
	if err := h.ipAllocator.GetUsage(ipPool.Spec.NetworkName); err != nil {
		klog.Infof("initialize the ipam module for ippool %s/%s", ipPool.Namespace, ipPool.Name)
		if err := h.ipAllocator.NewIPSubnet(
			ipPool.Spec.NetworkName,
			ipPool.Spec.IPv4Config.CIDR,
			ipPool.Spec.IPv4Config.Pool.Start,
			ipPool.Spec.IPv4Config.Pool.End,
		); err != nil {
			return ipPool, err
		}

		for _, ip := range ipPool.Spec.IPv4Config.Pool.Exclude {
			if _, err := h.ipAllocator.AllocateIP(ipPool.Spec.NetworkName, ip.String()); err != nil {
				return ipPool, err
			}
			klog.Infof("excluded ip %s was marked as allocated in ipam module %s", ip, ipPool.Spec.NetworkName)
		}

		for ip := range ipPool.Status.IPv4.Allocated {
			if _, err := h.ipAllocator.AllocateIP(ipPool.Spec.NetworkName, ip); err != nil {
				return ipPool, err
			}
			klog.Infof("previously allocated ip %s was re-added into ipam module %s", ip, ipPool.Spec.NetworkName)
		}
	}

	klog.Infof("ipam %s for ippool %s/%s has been initialized", ipPool.Spec.NetworkName, ipPool.Namespace, ipPool.Name)

	// TODO: mark excluded IP addresses in the IPAM module

	ipPoolCpy := ipPool.DeepCopy()
	ipPoolCpy.Status.LastUpdate = metav1.Now()

	// Pool usage status
	used, err := h.ipAllocator.GetUsed(ipPool.Spec.NetworkName)
	if err != nil {
		return ipPool, err
	}
	ipPoolCpy.Status.IPv4.Used = used
	available, err := h.ipAllocator.GetAvailable(ipPool.Spec.NetworkName)
	if err != nil {
		return ipPool, err
	}
	ipPoolCpy.Status.IPv4.Available = available

	if ipPool.Status.IPv4.Allocated == nil {
		allocated := make(map[string]string)
		for _, v := range ipPool.Spec.IPv4Config.Pool.Exclude {
			allocated[v.String()] = excludedMark
		}
		ipPoolCpy.Status.IPv4.Allocated = allocated
	}

	if !reflect.DeepEqual(ipPoolCpy, ipPool) {
		klog.Infof("update ippool %s/%s", ipPool.Namespace, ipPool.Name)
		return h.ippoolClient.UpdateStatus(ipPoolCpy)
	}

	return ipPool, nil
}
