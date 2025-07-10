package vm

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubevirtv1 "kubevirt.io/api/core/v1"

	networkv1 "github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
)

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

type vmBuilder struct {
	vm *kubevirtv1.VirtualMachine
}

func newVMBuilder(namespace, name string) *vmBuilder {
	return &vmBuilder{
		vm: &kubevirtv1.VirtualMachine{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
		},
	}
}

// WithInterface adds a network interface to the VM with the specified MAC address, NIC name, and network name.
func (b *vmBuilder) WithInterface(macAddress, nicName string) *vmBuilder {
	if b.vm.Spec.Template == nil {
		b.vm.Spec.Template = &kubevirtv1.VirtualMachineInstanceTemplateSpec{}
	}

	b.vm.Spec.Template.Spec.Domain.Devices.Interfaces = append(b.vm.Spec.Template.Spec.Domain.Devices.Interfaces, kubevirtv1.Interface{
		Name:       nicName,
		MacAddress: macAddress,
	})

	return b
}

// WithNetwork adds a network configuration to the VM.
// If networkName is empty, it defaults to a Pod network.
func (b *vmBuilder) WithNetwork(nicName, networkName string) *vmBuilder {
	if b.vm.Spec.Template == nil {
		b.vm.Spec.Template = &kubevirtv1.VirtualMachineInstanceTemplateSpec{}
	}

	var ns kubevirtv1.NetworkSource
	if networkName == "" {
		ns = kubevirtv1.NetworkSource{
			Pod: &kubevirtv1.PodNetwork{},
		}
	} else {
		ns = kubevirtv1.NetworkSource{
			Multus: &kubevirtv1.MultusNetwork{
				NetworkName: networkName,
			},
		}
	}

	b.vm.Spec.Template.Spec.Networks = append(b.vm.Spec.Template.Spec.Networks, kubevirtv1.Network{
		Name:          nicName,
		NetworkSource: ns,
	})

	return b
}

func (b *vmBuilder) Build() *kubevirtv1.VirtualMachine {
	return b.vm
}
