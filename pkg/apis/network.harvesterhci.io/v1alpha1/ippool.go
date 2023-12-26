package v1alpha1

import (
	"net"

	"github.com/rancher/wrangler/pkg/condition"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	Registered condition.Cond = "Registered"
	Ready      condition.Cond = "Ready"
	Stopped    condition.Cond = "Stopped"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=ipl;ipls,scope=Namespaced
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
	ServerIP     net.IP   `json:"serverIP,omitempty"`
	CIDR         string   `json:"cidr,omitempty"`
	Pool         Pool     `json:"pool,omitempty"`
	Router       net.IP   `json:"router,omitempty"`
	DNS          []net.IP `json:"dns,omitempty"`
	DomainName   string   `json:"domainName,omitempty"`
	DomainSearch []string `json:"domainSearch,omitempty"`
	NTP          []string `json:"ntp,omitempty"`
	LeaseTime    int      `json:"leaseTime,omitempty"`
}

type Pool struct {
	Start   net.IP   `json:"start,omitempty"`
	End     net.IP   `json:"end,omitempty"`
	Exclude []net.IP `json:"exclude,omitempty"`
}

type IPPoolStatus struct {
	LastUpdate            metav1.Time `json:"lastUpdate,omitempty"`
	LastUpdateBeforeStart metav1.Time `json:"lastUpdateBeforeStart,omitempty"`
	IPv4                  IPv4Status  `json:"ipv4,omitempty"`
	// +optional
	Conditions []Condition `json:"conditions,omitempty"`
}

type IPv4Status struct {
	Allocated map[string]string `json:"allocated,omitempty"`
	Used      int               `json:"used,omitempty"`
	Available int               `json:"available,omitempty"`
}

type Condition struct {
	// Type of the condition.
	Type condition.Cond `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Human-readable message indicating details about last transition
	Message string `json:"message,omitempty"`
}
