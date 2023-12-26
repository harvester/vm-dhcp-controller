package ippool

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	networkv1 "github.com/starbops/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/starbops/vm-dhcp-controller/pkg/config"
	ctlnetworkv1 "github.com/starbops/vm-dhcp-controller/pkg/generated/controllers/network.harvesterhci.io/v1alpha1"
)

const controllerName = "vm-dhcp-ippool-controller"

type Handler struct {
	poolRef types.NamespacedName

	ippoolClient ctlnetworkv1.IPPoolClient
	ippoolCache  ctlnetworkv1.IPPoolCache
}

func Register(ctx context.Context, management *config.Management) error {
	ippools := management.HarvesterNetworkFactory.Network().V1alpha1().IPPool()

	handler := &Handler{
		poolRef: management.Options.PoolRef,

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

	// The agent only focuses on the designated IPPool object
	ipPoolNamespacedName := types.NamespacedName{
		Namespace: ipPool.Namespace,
		Name:      ipPool.Name,
	}
	if ipPoolNamespacedName != h.poolRef {
		return ipPool, nil
	}

	klog.Infof("ippool configuration %s/%s has been changed: %+v", ipPool.Namespace, ipPool.Name, ipPool.Spec.IPv4Config)

	return ipPool, nil
}

func (h *Handler) OnRemove(key string, ipPool *networkv1.IPPool) (*networkv1.IPPool, error) {
	if ipPool == nil {
		return nil, nil
	}

	klog.Infof("ippool configuration %s/%s has been removed", ipPool.Namespace, ipPool.Name)

	return ipPool, nil
}
