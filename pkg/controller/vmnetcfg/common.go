package vmnetcfg

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	networkv1 "github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
)

func updateAllNetworkConfigState(ncStatuses []networkv1.NetworkConfigStatus, state networkv1.NetworkConfigState) {
	for i := range ncStatuses {
		ncStatuses[i].State = state
	}
}

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

func setInSyncedCondition(vmNetCfg *networkv1.VirtualMachineNetworkConfig, status corev1.ConditionStatus, reason, message string) {
	networkv1.InSynced.SetStatus(vmNetCfg, string(status))
	networkv1.InSynced.Reason(vmNetCfg, reason)
	networkv1.InSynced.Message(vmNetCfg, message)
}

type VmNetCfgBuilder struct {
	vmNetCfg *networkv1.VirtualMachineNetworkConfig
}

func NewVmNetCfgBuilder(namespace, name string) *VmNetCfgBuilder {
	return &VmNetCfgBuilder{
		vmNetCfg: &networkv1.VirtualMachineNetworkConfig{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
		},
	}
}

func (b *VmNetCfgBuilder) Label(key, value string) *VmNetCfgBuilder {
	if b.vmNetCfg.Labels == nil {
		b.vmNetCfg.Labels = make(map[string]string)
	}
	b.vmNetCfg.Labels[key] = value
	return b
}

func (b *VmNetCfgBuilder) OwnerRef(owner metav1.OwnerReference) *VmNetCfgBuilder {
	if b.vmNetCfg.OwnerReferences == nil {
		b.vmNetCfg.OwnerReferences = []metav1.OwnerReference{}
	}
	b.vmNetCfg.OwnerReferences = append(b.vmNetCfg.OwnerReferences, owner)
	return b
}

func (b *VmNetCfgBuilder) WithVMName(name string) *VmNetCfgBuilder {
	b.vmNetCfg.Spec.VMName = name
	return b
}

func (b *VmNetCfgBuilder) Paused() *VmNetCfgBuilder {
	b.vmNetCfg.Spec.Paused = func(b bool) *bool { return &b }(true)
	return b
}

func (b *VmNetCfgBuilder) UnPaused() *VmNetCfgBuilder {
	b.vmNetCfg.Spec.Paused = func(b bool) *bool { return &b }(false)
	return b
}

func (b *VmNetCfgBuilder) WithNetworkConfig(ipAddress, macAddress, networkName string) *VmNetCfgBuilder {
	var ip *string
	if ipAddress != "" {
		ip = &ipAddress
	}
	nc := networkv1.NetworkConfig{
		IPAddress:   ip,
		MACAddress:  macAddress,
		NetworkName: networkName,
	}
	b.vmNetCfg.Spec.NetworkConfigs = append(b.vmNetCfg.Spec.NetworkConfigs, nc)
	return b
}

func (b *VmNetCfgBuilder) WithNetworkConfigStatus(ipAddress, macAddress, networkName string, state networkv1.NetworkConfigState) *VmNetCfgBuilder {
	ncStatus := networkv1.NetworkConfigStatus{
		AllocatedIPAddress: ipAddress,
		MACAddress:         macAddress,
		NetworkName:        networkName,
		State:              state,
	}
	b.vmNetCfg.Status.NetworkConfigs = append(b.vmNetCfg.Status.NetworkConfigs, ncStatus)
	return b
}

func (b *VmNetCfgBuilder) AllocatedCondition(status corev1.ConditionStatus, reason, message string) *VmNetCfgBuilder {
	setAllocatedCondition(b.vmNetCfg, status, reason, message)
	return b
}

func (b *VmNetCfgBuilder) DisabledCondition(status corev1.ConditionStatus, reason, message string) *VmNetCfgBuilder {
	setDisabledCondition(b.vmNetCfg, status, reason, message)
	return b
}

func (b *VmNetCfgBuilder) InSyncedCondition(status corev1.ConditionStatus, reason, message string) *VmNetCfgBuilder {
	setInSyncedCondition(b.vmNetCfg, status, reason, message)
	return b
}

func (b *VmNetCfgBuilder) Build() *networkv1.VirtualMachineNetworkConfig {
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
	b.vmNetCfgStatus.NetworkConfigs = append(b.vmNetCfgStatus.NetworkConfigs, ncStatus)
	return b
}

func (b *vmNetCfgStatusBuilder) InSyncedCondition(status corev1.ConditionStatus, reason, message string) *vmNetCfgStatusBuilder {
	networkv1.InSynced.SetStatus(&b.vmNetCfgStatus, string(status))
	networkv1.InSynced.Reason(&b.vmNetCfgStatus, reason)
	networkv1.InSynced.Message(&b.vmNetCfgStatus, message)
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
