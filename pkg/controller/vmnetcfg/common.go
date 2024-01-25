package vmnetcfg

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	networkv1 "github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
)

func setAllocatedCondition(vmNetCfg *networkv1.VirtualMachineNetworkConfig, status corev1.ConditionStatus, reason, message string) {
	networkv1.Allocated.SetStatus(vmNetCfg, string(status))
	networkv1.Allocated.Reason(vmNetCfg, reason)
	networkv1.Allocated.Message(vmNetCfg, message)
}

func setDisabledCondition(vmNetCfg *networkv1.VirtualMachineNetworkConfig, status corev1.ConditionStatus, reason, message string) {
	networkv1.Disabled.SetStatus(vmNetCfg, string(status))
	networkv1.Disabled.Reason(vmNetCfg, reason)
	networkv1.Disabled.Message(vmNetCfg, message)
}

type vmNetCfgBuilder struct {
	vmNetCfg *networkv1.VirtualMachineNetworkConfig
}

func newVmNetCfgBuilder(namespace, name string) *vmNetCfgBuilder {
	return &vmNetCfgBuilder{
		vmNetCfg: &networkv1.VirtualMachineNetworkConfig{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
		},
	}
}

func (b *vmNetCfgBuilder) Paused() *vmNetCfgBuilder {
	b.vmNetCfg.Spec.Paused = func(b bool) *bool { return &b }(true)
	return b
}

func (b *vmNetCfgBuilder) UnPaused() *vmNetCfgBuilder {
	b.vmNetCfg.Spec.Paused = func(b bool) *bool { return &b }(false)
	return b
}

func (b *vmNetCfgBuilder) WithNetworkConfig(ipAddress, macAddress, networkName string) *vmNetCfgBuilder {
	var ip *string
	if ipAddress != "" {
		ip = &ipAddress
	}
	nc := networkv1.NetworkConfig{
		IPAddress:   ip,
		MACAddress:  macAddress,
		NetworkName: networkName,
	}
	b.vmNetCfg.Spec.NetworkConfig = append(b.vmNetCfg.Spec.NetworkConfig, nc)
	return b
}

func (b *vmNetCfgBuilder) WithNetworkConfigStatus(ipAddress, macAddress, networkName string, state networkv1.NetworkConfigState) *vmNetCfgBuilder {
	ncStatus := networkv1.NetworkConfigStatus{
		AllocatedIPAddress: ipAddress,
		MACAddress:         macAddress,
		NetworkName:        networkName,
		State:              state,
	}
	b.vmNetCfg.Status.NetworkConfig = append(b.vmNetCfg.Status.NetworkConfig, ncStatus)
	return b
}

func (b *vmNetCfgBuilder) AllocatedCondition(status corev1.ConditionStatus, reason, message string) *vmNetCfgBuilder {
	setAllocatedCondition(b.vmNetCfg, status, reason, message)
	return b
}

func (b *vmNetCfgBuilder) DisabledCondition(status corev1.ConditionStatus, reason, message string) *vmNetCfgBuilder {
	setDisabledCondition(b.vmNetCfg, status, reason, message)
	return b
}

func (b *vmNetCfgBuilder) Build() *networkv1.VirtualMachineNetworkConfig {
	return b.vmNetCfg
}

type vmNetCfgStatusBuilder struct {
	vmNetCfgStatus networkv1.VirtualMachineNetworkConfigStatus
}

func newVmNetCfgStatusBuilder() *vmNetCfgStatusBuilder {
	return &vmNetCfgStatusBuilder{
		vmNetCfgStatus: networkv1.VirtualMachineNetworkConfigStatus{},
	}
}

func (b *vmNetCfgStatusBuilder) WithNetworkConfigStatus(ipAddress, macAddress, networkName string, state networkv1.NetworkConfigState) *vmNetCfgStatusBuilder {
	ncStatus := networkv1.NetworkConfigStatus{
		AllocatedIPAddress: ipAddress,
		MACAddress:         macAddress,
		NetworkName:        networkName,
		State:              state,
	}
	b.vmNetCfgStatus.NetworkConfig = append(b.vmNetCfgStatus.NetworkConfig, ncStatus)
	return b
}

func (b *vmNetCfgStatusBuilder) Build() networkv1.VirtualMachineNetworkConfigStatus {
	return b.vmNetCfgStatus
}

func SanitizeStatus(status *networkv1.VirtualMachineNetworkConfigStatus) {
	for i := range status.Conditions {
		status.Conditions[i].LastTransitionTime = ""
		status.Conditions[i].LastUpdateTime = ""
	}
}
