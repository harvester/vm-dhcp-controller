package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=ipl;ipls,scope=Cluster
// +kubebuilder:printcolumn:name="NETWORK",type=string,JSONPath=`.spec.networkName`
// +kubebuilder:printcolumn:name="AVAILABLE",type=string,JSONPath=`.status.ipv4.available`
// +kubebuilder:printcolumn:name="USED",type=string,JSONPath=`.status.ipv4.used`
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=`.metadata.creationTimestamp`

type IPPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IPPoolSpec   `json:"spec,omitempty"`
	Status IPPoolStatus `json:"status,omitempty"`
}

type IPPoolSpec struct {
	IPv4Config  IPv4Config `json:"ipv4Config,omitempty"`
	NetworkName string     `json:"networkName,omitempty"`
}

type IPv4Config struct {
	ServerIP     string   `json:"serverIP,omitempty"`
	Subnet       string   `json:"subnet,omitempty"`
	Pool         Pool     `json:"pool,omitempty"`
	Router       string   `json:"router,omitempty"`
	DNS          []string `json:"dns,omitempty"`
	DomainName   string   `json:"domainName,omitempty"`
	DomainSearch []string `json:"domainSearch,omitempty"`
	NTP          []string `json:"ntp,omitempty"`
	LeaseTime    int      `json:"leaseTime,omitempty"`
}

type Pool struct {
	Start   string   `json:"start,omitempty"`
	End     string   `json:"end,omitempty"`
	Exclude []string `json:"exclude,omitempty"`
}

type IPPoolStatus struct {
	LastUpdate            metav1.Time `json:"lastUpdate,omitempty"`
	LastUpdateBeforeStart metav1.Time `json:"lastUpdateBefoRestart,omitempty"`
	IPv4                  IPv4Status  `json:"ipv4,omitempty"`
}

type IPv4Status struct {
	Allocated map[string]string `json:"allocated,omitempty"`
	Used      int               `json:"used,omitempty"`
	Available int               `json:"available,omitempty"`
}
