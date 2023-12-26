package vmnetcfg

import (
	"context"

	"k8s.io/klog/v2"

	networkv1 "github.com/starbops/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/starbops/vm-dhcp-controller/pkg/config"
	ctlnetworkv1 "github.com/starbops/vm-dhcp-controller/pkg/generated/controllers/network.harvesterhci.io/v1alpha1"
)

const controllerName = "vm-dhcp-vmnetcfg-controller"

type Handler struct {
	vmnetcfgClient ctlnetworkv1.VirtualMachineNetworkConfigClient
	vmnetcfgCache  ctlnetworkv1.VirtualMachineNetworkConfigCache
	ippoolClient   ctlnetworkv1.IPPoolClient
	ippoolCache    ctlnetworkv1.IPPoolCache
}

func Register(ctx context.Context, management *config.Management) error {
	vmnetcfgs := management.HarvesterNetworkFactory.Network().V1alpha1().VirtualMachineNetworkConfig()
	ippools := management.HarvesterNetworkFactory.Network().V1alpha1().IPPool()

	handler := &Handler{
		vmnetcfgClient: vmnetcfgs,
		vmnetcfgCache:  vmnetcfgs.Cache(),
		ippoolClient:   ippools,
		ippoolCache:    ippools.Cache(),
	}

	vmnetcfgs.OnChange(ctx, controllerName, handler.OnChange)
	vmnetcfgs.OnRemove(ctx, controllerName, handler.OnRemove)

	return nil
}

func (h *Handler) OnChange(key string, vmNetCfg *networkv1.VirtualMachineNetworkConfig) (*networkv1.VirtualMachineNetworkConfig, error) {
	if vmNetCfg == nil || vmNetCfg.DeletionTimestamp != nil {
		return nil, nil
	}

	klog.Infof("vmnetcfg configuration %s/%s has been changed: %+v", vmNetCfg.Namespace, vmNetCfg.Name, vmNetCfg.Spec.NetworkConfig)

	return vmNetCfg, nil
}

func (h *Handler) OnRemove(key string, vmNetCfg *networkv1.VirtualMachineNetworkConfig) (*networkv1.VirtualMachineNetworkConfig, error) {
	if vmNetCfg == nil {
		return nil, nil
	}

	klog.Infof("vmnetcfg configuration %s/%s has been removed", vmNetCfg.Namespace, vmNetCfg.Name)

	return vmNetCfg, nil
}
