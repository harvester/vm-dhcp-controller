package v1alpha1

import (
	"net"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=vmnetcfg;vmnetcfgs,scope=Namespaced
// +kubebuilder:printcolumn:name="VMNAME",type=string,JSONPath=`.spec.vmName`
// +kubebuilder:printcolumn:name="NETWORK",type=string,JSONPath=`.spec.networkConfig.networkName`
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
}

type NetworkConfig struct {
	IPAddress   net.IP           `json:"ipAddress,omitempty"`
	MACAddress  net.HardwareAddr `json:"macAddress,omitempty"`
	NetworkName string           `json:"networkName,omitempty"`
}

type VirtualMachineNetworkConfigStatus struct {
	NetworkConfig []NetworkConfigStatus `json:"networkConfig,omitempty"`
}

type NetworkConfigStatus struct {
	MACAddress  net.HardwareAddr `json:"macAddress,omitempty"`
	NetworkName string           `json:"networkName,omitempty"`
	Status      string           `json:"status,omitempty"`
	Message     string           `json:"message,omitempty"`
}
