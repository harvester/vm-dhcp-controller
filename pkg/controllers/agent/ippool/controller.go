package ippool

import (
	"context"
	"os"
	"strconv"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	networkv1 "github.com/starbops/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/starbops/vm-dhcp-controller/pkg/config"
	"github.com/starbops/vm-dhcp-controller/pkg/dhcp"
	ctlnetworkv1 "github.com/starbops/vm-dhcp-controller/pkg/generated/controllers/network.harvesterhci.io/v1alpha1"
)

const (
	controllerName = "vm-dhcp-ippool-controller"

	PIDFilePath = "/tmp/dhcpd.pid"
)

type Handler struct {
	ctx context.Context

	poolRef types.NamespacedName

	dhcpAllocator *dhcp.DHCPAllocator

	ippoolClient ctlnetworkv1.IPPoolClient
	ippoolCache  ctlnetworkv1.IPPoolCache
}

func Register(ctx context.Context, management *config.Management) error {
	ippools := management.HarvesterNetworkFactory.Network().V1alpha1().IPPool()

	handler := &Handler{
		ctx: ctx,

		poolRef: management.Options.PoolRef,

		dhcpAllocator: dhcp.New(),

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
	if networkv1.Registered.IsTrue(ipPool) && networkv1.Ready.GetStatus(ipPool) == "" {
		klog.Infof("start the embedded dhcp server for ippool %s/%s", ipPool.Namespace, ipPool.Name)
		if err := h.dhcpAllocator.Run(h.ctx, "eth1"); err != nil {
			return ipPool, err
		}

		// Touch pid file for the readiness probe
		pid := os.Getpid()
		if err := os.WriteFile(PIDFilePath, []byte(strconv.Itoa(pid)), 0644); err != nil {
			return ipPool, err
		}

		klog.Infof("embedded dhcp server for ippool %s/%s is ready", ipPool.Namespace, ipPool.Name)

		return ipPool, nil
	}

	return ipPool, nil
}

func (h *Handler) OnRemove(key string, ipPool *networkv1.IPPool) (*networkv1.IPPool, error) {
	if ipPool == nil {
		return nil, nil
	}

	klog.Infof("ippool configuration %s/%s has been removed", ipPool.Namespace, ipPool.Name)

	return ipPool, nil
}
