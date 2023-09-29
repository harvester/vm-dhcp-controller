package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type VirtualMachineNetworkConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualMachineNetworkConfigSpec   `json:"spec,omitempty"`
	Status VirtualMachineNetworkConfigStatus `json:"status,omitempty"`
}

type VirtualMachineNetworkConfigSpec struct {
	VMName        string          `json:"vmname,omitempty"`
	NetworkConfig []NetworkConfig `json:"networkconfig,omitempty"`
}

type NetworkConfig struct {
	IPAddress   string `json:"ipaddress,omitempty"`
	MACAddress  string `json:"macaddress,omitempty"`
	NetworkName string `json:"networkname,omitempty"`
}

type VirtualMachineNetworkConfigStatus struct {
	NetworkConfig []NetworkConfigStatus `json:"networkconfig,omitempty"`
}

type NetworkConfigStatus struct {
	MACAddress  string `json:"macaddress,omitempty"`
	NetworkName string `json:"networkname,omitempty"`
	Status      string `json:"status,omitempty"`
	Message     string `json:"message,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type VirtualMachineNetworkConfigList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata.
	metav1.ListMeta `json:"metadata,omitempty"`
	// List of Fips.
	Items []VirtualMachineNetworkConfig `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type IPPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IPPoolSpec   `json:"spec,omitempty"`
	Status IPPoolStatus `json:"status,omitempty"`
}

type IPPoolSpec struct {
	IPv4Config  IPv4Config `json:"ipv4config,omitempty"`
	NetworkName string     `json:"networkname,omitempty"`
}

type IPv4Config struct {
	ServerIP     string   `json:"serverip,omitempty"`
	Subnet       string   `json:"subnet,omitempty"`
	Pool         Pool     `json:"pool,omitempty"`
	Router       string   `json:"router,omitempty"`
	DNS          []string `json:"dns,omitempty"`
	DomainName   string   `json:"domainname,omitempty"`
	DomainSearch []string `json:"domainsearch,omitempty"`
	LeaseTime    int      `json:"leasetime,omitempty"`
}

type Pool struct {
	Start   string   `json:"start,omitempty"`
	End     string   `json:"end,omitempty"`
	Exclude []string `json:"exclude,omitempty"`
}

type IPPoolStatus struct {
	LastUpdate            metav1.Time `json:"lastupdate,omitempty"`
	LastUpdateBeforeStart metav1.Time `json:"lastupdatebeforestart,omitempty"`
	IPv4                  IPv4Status  `json:"ipv4,omitempty"`
}

type IPv4Status struct {
	Allocated map[string]string `json:"allocated,omitempty"`
	Used      int               `json:"used,omitempty"`
	Available int               `json:"available,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type IPPoolList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata.
	metav1.ListMeta `json:"metadata,omitempty"`
	// List of Fips.
	Items []IPPool `json:"items"`
}
