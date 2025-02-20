package vm

import (
	"context"
	"reflect"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubevirtv1 "kubevirt.io/api/core/v1"

	networkv1 "github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/harvester/vm-dhcp-controller/pkg/config"
	ctlkubevirtv1 "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/kubevirt.io/v1"
	ctlnetworkv1 "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/network.harvesterhci.io/v1alpha1"
)

const (
	controllerName = "vm-dhcp-vm-controller"

	vmLabelKey = "harvesterhci.io/vmName"
)

type Handler struct {
	vmController   ctlkubevirtv1.VirtualMachineController
	vmClient       ctlkubevirtv1.VirtualMachineClient
	vmCache        ctlkubevirtv1.VirtualMachineCache
	vmnetcfgClient ctlnetworkv1.VirtualMachineNetworkConfigClient
	vmnetcfgCache  ctlnetworkv1.VirtualMachineNetworkConfigCache
}

func Register(ctx context.Context, management *config.Management) error {
	vms := management.KubeVirtFactory.Kubevirt().V1().VirtualMachine()
	vmnetcfgs := management.HarvesterNetworkFactory.Network().V1alpha1().VirtualMachineNetworkConfig()

	handler := &Handler{
		vmController:   vms,
		vmClient:       vms,
		vmCache:        vms.Cache(),
		vmnetcfgClient: vmnetcfgs,
		vmnetcfgCache:  vmnetcfgs.Cache(),
	}

	vms.OnChange(ctx, controllerName, handler.OnChange)

	return nil
}

func (h *Handler) OnChange(key string, vm *kubevirtv1.VirtualMachine) (*kubevirtv1.VirtualMachine, error) {
	if vm == nil || vm.DeletionTimestamp != nil {
		return nil, nil
	}

	logrus.Debugf("(vm.OnChange) vm configuration %s/%s has been changed", vm.Namespace, vm.Name)

	ncm := make(map[string]networkv1.NetworkConfig, 1)

	// Construct initial network config map
	for _, nic := range vm.Spec.Template.Spec.Domain.Devices.Interfaces {
		if nic.MacAddress == "" {
			continue
		}
		ncm[nic.Name] = networkv1.NetworkConfig{
			MACAddress: nic.MacAddress,
		}
	}

	// Update network name for each network config if it's of type Multus
	for _, network := range vm.Spec.Template.Spec.Networks {
		if network.NetworkSource.Multus == nil {
			continue
		}
		nc, ok := ncm[network.Name]
		if !ok {
			continue
		}
		nc.NetworkName = network.Multus.NetworkName
		ncm[network.Name] = nc
	}

	// Remove incomplete network configs
	for i, nc := range ncm {
		if nc.NetworkName == "" {
			delete(ncm, i)
		}
	}

	vmNetCfg := prepareVmNetCfg(vm, ncm)

	oldVmNetCfg, err := h.vmnetcfgCache.Get(vm.Namespace, vm.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Infof("(vm.OnChange) create vmnetcfg for vm %s", key)
			if _, err := h.vmnetcfgClient.Create(vmNetCfg); err != nil {
				return vm, err
			}
			return vm, nil
		}
		return vm, err
	}

	logrus.Debugf("(vm.OnChange) vmnetcfg for vm %s already exists", key)

	vmNetCfgCpy := oldVmNetCfg.DeepCopy()
	vmNetCfgCpy.Spec.NetworkConfigs = vmNetCfg.Spec.NetworkConfigs

	// The following block is a two-step process. Ideally,
	// 1. if the network config of the VirtualMachine has been changed, update the status of the VirtualMachineNetworkConfig
	//   to out-of-sync so that the vmnetcfg-controller can handle it accordingly, and
	// 2. since the spec of the VirtualMachineNetworkConfig hasn't been changed, update it to reflect the new network config.
	// This is to throttle the vmnetcfg-controller and to avoid allocate-before-deallocate from happening.
	if !reflect.DeepEqual(vmNetCfgCpy.Spec.NetworkConfigs, oldVmNetCfg.Spec.NetworkConfigs) {
		if networkv1.InSynced.IsFalse(oldVmNetCfg) {
			logrus.Infof("(vm.OnChange) vmnetcfg %s/%s is deemed out-of-sync, updating it", vmNetCfgCpy.Namespace, vmNetCfgCpy.Name)
			if _, err := h.vmnetcfgClient.Update(vmNetCfgCpy); err != nil {
				return vm, err
			}
			return vm, nil
		}

		logrus.Infof("(vm.OnChange) update vmnetcfg %s/%s status as out-of-sync due to network config changes", vmNetCfgCpy.Namespace, vmNetCfgCpy.Name)

		// Mark the VirtualMachineNetworkConfig as out-of-sync so that the vmnetcfg-controller can handle it accordingly
		networkv1.InSynced.SetStatus(vmNetCfgCpy, string(corev1.ConditionFalse))
		networkv1.InSynced.Reason(vmNetCfgCpy, "NetworkConfigChanged")
		networkv1.InSynced.Message(vmNetCfgCpy, "Network configuration of the upstrem virtual machine has been changed")

		if _, err := h.vmnetcfgClient.UpdateStatus(vmNetCfgCpy); err != nil {
			return vm, err
		}

		// Enqueue the VirtualMachine in order to update the network config of its corresponding VirtualMachineNetworkConfig
		h.vmController.Enqueue(vm.Namespace, vm.Name)
	}

	return vm, nil
}

func prepareVmNetCfg(vm *kubevirtv1.VirtualMachine, ncm map[string]networkv1.NetworkConfig) *networkv1.VirtualMachineNetworkConfig {
	sets := labels.Set{
		vmLabelKey: vm.Name,
	}

	ncs := make([]networkv1.NetworkConfig, 0, len(ncm))
	for _, nc := range ncm {
		ncs = append(ncs, nc)
	}

	return &networkv1.VirtualMachineNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Labels:    sets,
			Name:      vm.Name,
			Namespace: vm.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: vm.APIVersion,
					Kind:       vm.Kind,
					Name:       vm.Name,
					UID:        vm.UID,
				},
			},
		},
		Spec: networkv1.VirtualMachineNetworkConfigSpec{
			VMName:         vm.Name,
			NetworkConfigs: ncs,
		},
	}
}
