package v1alpha1

import (
	"net"

	"github.com/rancher/wrangler/pkg/condition"
	"github.com/rancher/wrangler/pkg/genericcondition"
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
	ServerIP net.IP `json:"serverIP,omitempty"`
	CIDR     string `json:"cidr,omitempty"`
	Pool     Pool   `json:"pool,omitempty"`

	// +optional
	Router net.IP `json:"router,omitempty"`

	// +optional
	DNS []net.IP `json:"dns,omitempty"`

	// +optional
	DomainName *string `json:"domainName,omitempty"`

	// +optional
	DomainSearch []string `json:"domainSearch,omitempty"`

	// +optional
	NTP []string `json:"ntp,omitempty"`

	// +optional
	LeaseTime *int `json:"leaseTime,omitempty"`
}

type Pool struct {
	Start net.IP `json:"start,omitempty"`
	End   net.IP `json:"end,omitempty"`

	// +optional
	Exclude []net.IP `json:"exclude,omitempty"`
}

type IPPoolStatus struct {
	LastUpdate            metav1.Time `json:"lastUpdate,omitempty"`
	LastUpdateBeforeStart metav1.Time `json:"lastUpdateBeforeStart,omitempty"`

	// +optional
	IPv4 *IPv4Status `json:"ipv4,omitempty"`

	// +optional
	AgentPodRef *PodReference `json:"agentPodRef,omitempty"`

	// // +optional
	// Conditions []Condition `json:"conditions,omitempty"`
	// Conditions is a list of Wrangler conditions that describe the state
	// of the IPPool.
	Conditions []genericcondition.GenericCondition `json:"conditions,omitempty"`
}

type IPv4Status struct {
	Allocated map[string]string `json:"allocated,omitempty"`
	Used      int               `json:"used,omitempty"`
	Available int               `json:"available,omitempty"`
}

type PodReference struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
}
