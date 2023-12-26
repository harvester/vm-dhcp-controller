package vm

import (
	"context"

	"k8s.io/klog/v2"
	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/starbops/vm-dhcp-controller/pkg/config"
	ctlkubevirtv1 "github.com/starbops/vm-dhcp-controller/pkg/generated/controllers/kubevirt.io/v1"
	ctlnetworkv1 "github.com/starbops/vm-dhcp-controller/pkg/generated/controllers/network.harvesterhci.io/v1alpha1"
)

const controllerName = "vm-dhcp-vm-controller"

type Handler struct {
	vmClient       ctlkubevirtv1.VirtualMachineClient
	vmCache        ctlkubevirtv1.VirtualMachineCache
	vmnetcfgClient ctlnetworkv1.VirtualMachineNetworkConfigClient
	vmnetcfgCache  ctlnetworkv1.VirtualMachineNetworkConfigCache
}

func Register(ctx context.Context, management *config.Management) error {
	vms := management.KubeVirtFactory.Kubevirt().V1().VirtualMachine()
	vmnetcfgs := management.HarvesterNetworkFactory.Network().V1alpha1().VirtualMachineNetworkConfig()

	handler := &Handler{
		vmClient:       vms,
		vmCache:        vms.Cache(),
		vmnetcfgClient: vmnetcfgs,
		vmnetcfgCache:  vmnetcfgs.Cache(),
	}

	vms.OnChange(ctx, controllerName, handler.OnChange)
	vms.OnRemove(ctx, controllerName, handler.OnRemove)

	return nil
}

func (h *Handler) OnChange(key string, vm *kubevirtv1.VirtualMachine) (*kubevirtv1.VirtualMachine, error) {
	if vm == nil || vm.DeletionTimestamp != nil {
		return nil, nil
	}

	klog.Infof("vm configuration %s/%s has been changed", vm.Namespace, vm.Name)

	return vm, nil
}

func (h *Handler) OnRemove(key string, vm *kubevirtv1.VirtualMachine) (*kubevirtv1.VirtualMachine, error) {
	if vm == nil {
		return nil, nil
	}

	klog.Infof("vm configuration %s/%s has been removed", vm.Namespace, vm.Name)

	return vm, nil
}
