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

	nic         = "eth1"
	PIDFilePath = "/tmp/dhcpd.pid"
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
	ippools.OnRemove(ctx, controllerName, handler.OnRemove)

	return nil
}

func (h *Handler) OnChange(key string, ipPool *networkv1.IPPool) (*networkv1.IPPool, error) {
	if ipPool == nil || ipPool.DeletionTimestamp != nil {
		return nil, nil
	}

	// The agent only focuses on one target IPPool object
	ipPoolNamespacedName := types.NamespacedName{
		Namespace: ipPool.Namespace,
		Name:      ipPool.Name,
	}
	if ipPoolNamespacedName != h.poolRef {
		return ipPool, nil
	}

	klog.Infof("ippool configuration %s has been changed: %+v", ipPoolNamespacedName, ipPool.Spec.IPv4Config)

	// Run embedded DHCP server

	if h.dryRun {
		klog.Info("dry-run mode")
		if err := h.dhcpAllocator.DryRun(h.ctx, nic); err != nil {
			return ipPool, err
		}
	} else {
		if _, err := os.Stat(PIDFilePath); err != nil {
			klog.Infof("start embedded dhcp server for ippool %s/%s", ipPool.Namespace, ipPool.Name)
			if err := h.dhcpAllocator.Run(h.ctx, nic); err != nil {
				return ipPool, err
			}

			// Touch pid file for the readiness probe
			pid := os.Getpid()
			if err := os.WriteFile(PIDFilePath, []byte(strconv.Itoa(pid)), 0644); err != nil {
				return ipPool, err
			}
		}
		klog.Infof("embedded dhcp server for ippool %s/%s is ready", ipPool.Namespace, ipPool.Name)
	}

	// Construct IPAM from IPPool spec

	// if err := h.ipAllocator.GetUsage(ipPool.Spec.NetworkName); err != nil {
	klog.Infof("initialize ipam for ippool %s/%s", ipPool.Namespace, ipPool.Name)
	if err := h.ipAllocator.NewIPSubnet(
		ipPool.Spec.NetworkName,
		ipPool.Spec.IPv4Config.CIDR,
		ipPool.Spec.IPv4Config.Pool.Start,
		ipPool.Spec.IPv4Config.Pool.End,
	); err != nil {
		return ipPool, err
	}
	// }

	// Revoke the excluded IP addresses in IPAM
	for _, ip := range ipPool.Spec.IPv4Config.Pool.Exclude {
		if err := h.ipAllocator.RevokeIP(ipPool.Spec.NetworkName, ip.String()); err != nil {
			return ipPool, err
		}
		klog.Infof("excluded ip %s was revoked in ipam %s", ip, ipPool.Spec.NetworkName)
	}

	// Construct IPAM from IPPool status

	if ipPool.Status.IPv4 != nil {
		for ip, mac := range ipPool.Status.IPv4.Allocated {
			if mac == ipam.ExcludedMark {
				continue
			}
			if _, err := h.ipAllocator.AllocateIP(ipPool.Spec.NetworkName, ip); err != nil {
				return ipPool, err
			}
			klog.Infof("previously allocated ip %s was re-allocated in ipam %s", ip, ipPool.Spec.NetworkName)
		}
	}

	klog.Infof("ipam %s for ippool %s/%s has been updated", ipPool.Spec.NetworkName, ipPool.Namespace, ipPool.Name)

	// Update IPPool status based on up-to-date IPAM

	ipPoolCpy := ipPool.DeepCopy()

	// ipv4Status := util.ExistedOrNew(ipPoolCpy.Status.IPv4, new(networkv1.IPv4Status)).(*networkv1.IPv4Status)
	ipv4Status := ipPoolCpy.Status.IPv4
	if ipv4Status == nil {
		ipv4Status = new(networkv1.IPv4Status)
	}

	used, err := h.ipAllocator.GetUsed(ipPool.Spec.NetworkName)
	if err != nil {
		return ipPool, err
	}
	ipv4Status.Used = used

	available, err := h.ipAllocator.GetAvailable(ipPool.Spec.NetworkName)
	if err != nil {
		return ipPool, err
	}
	ipv4Status.Available = available

	// TODO: consider previously allocated IP addresses recorded in the IPPool status

	// allocated := util.ExistedOrNew(ipv4Status.Allocated, map[string]string{}).(map[string]string)
	allocated := ipv4Status.Allocated
	if allocated == nil {
		allocated = make(map[string]string)
	}
	for _, v := range ipPool.Spec.IPv4Config.Pool.Exclude {
		allocated[v.String()] = ipam.ExcludedMark
	}
	ipv4Status.Allocated = allocated

	ipPoolCpy.Status.IPv4 = ipv4Status

	if !reflect.DeepEqual(ipPoolCpy, ipPool) {
		klog.Infof("update ippool %s/%s", ipPool.Namespace, ipPool.Name)
		ipPoolCpy.Status.LastUpdate = metav1.Now()
		return h.ippoolClient.UpdateStatus(ipPoolCpy)
	}

	return ipPool, nil
}

func (h *Handler) OnRemove(key string, ipPool *networkv1.IPPool) (*networkv1.IPPool, error) {
	if ipPool == nil {
		return nil, nil
	}

	klog.Infof("ippool configuration %s/%s has been removed", ipPool.Namespace, ipPool.Name)

	h.ipAllocator.DeleteIPSubnet(ipPool.Spec.NetworkName)

	return ipPool, nil
}
