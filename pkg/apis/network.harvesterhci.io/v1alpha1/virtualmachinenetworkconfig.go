package v1alpha1

import (
	"github.com/rancher/wrangler/pkg/condition"
	"github.com/rancher/wrangler/pkg/genericcondition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AllocatedState NetworkConfigState = "Allocated"
	PendingState   NetworkConfigState = "Pending"
)

var (
	Allocated condition.Cond = "Allocated"
	Disabled  condition.Cond = "Disabled"
)

type NetworkConfigState string

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=vmnetcfg;vmnetcfgs,scope=Namespaced
// +kubebuilder:printcolumn:name="VMNAME",type=string,JSONPath=`.spec.vmName`
// +kubebuilder:printcolumn:name="ALLOCATED",type=string,JSONPath=`.status.conditions[?(@.type=='Allocated')].status`
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=`.metadata.creationTimestamp`

type VirtualMachineNetworkConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualMachineNetworkConfigSpec   `json:"spec,omitempty"`
	Status VirtualMachineNetworkConfigStatus `json:"status,omitempty"`
}

type VirtualMachineNetworkConfigSpec struct {
	VMName        string          `json:"vmName,omitempty"`
	NetworkConfig []NetworkConfig `json:"networkConfig,omitempty"`

	// +optional
	Paused *bool `json:"paused,omitempty"`
}

type NetworkConfig struct {
	NetworkName string `json:"networkName,omitempty"`
	MACAddress  string `json:"macAddress,omitempty"`

	// +optional
	IPAddress *string `json:"ipAddress,omitempty"`
}

type VirtualMachineNetworkConfigStatus struct {
	NetworkConfig []NetworkConfigStatus `json:"networkConfig,omitempty"`

	// +optional
	Conditions []genericcondition.GenericCondition `json:"conditions,omitempty"`
}

type NetworkConfigStatus struct {
	AllocatedIPAddress string             `json:"allocatedIPAddress,omitempty"`
	MACAddress         string             `json:"macAddress,omitempty"`
	NetworkName        string             `json:"networkName,omitempty"`
	State              NetworkConfigState `json:"state,omitempty"`
}
