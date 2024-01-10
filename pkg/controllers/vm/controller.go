package vm

import (
	"context"
	"reflect"

	"github.com/sirupsen/logrus"
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

	return nil
}

func (h *Handler) OnChange(key string, vm *kubevirtv1.VirtualMachine) (*kubevirtv1.VirtualMachine, error) {
	if vm == nil || vm.DeletionTimestamp != nil {
		return nil, nil
	}

	logrus.Debugf("vm configuration %s/%s has been changed", vm.Namespace, vm.Name)

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
			logrus.Infof("create vmnetcfg for vm %s", key)
			if _, err := h.vmnetcfgClient.Create(vmNetCfg); err != nil {
				return vm, err
			}
			return vm, nil
		}
		return vm, err
	}

	logrus.Debugf("vmnetcfg for vm %s already exists", key)

	vmNetCfgCpy := oldVmNetCfg.DeepCopy()
	vmNetCfgCpy.Spec.NetworkConfig = vmNetCfg.Spec.NetworkConfig

	if !reflect.DeepEqual(vmNetCfgCpy, oldVmNetCfg) {
		logrus.Infof("update vmnetcfg %s/%s", vmNetCfgCpy.Namespace, vmNetCfgCpy.Name)
		if _, err := h.vmnetcfgClient.Update(vmNetCfgCpy); err != nil {
			return vm, err
		}
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
			VMName:        vm.Name,
			NetworkConfig: ncs,
		},
	}
}
