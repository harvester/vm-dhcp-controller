package v1alpha1

import (
	"github.com/rancher/wrangler/pkg/condition"
	"github.com/rancher/wrangler/pkg/genericcondition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var (
	Registered condition.Cond = "Registered"
	CacheReady condition.Cond = "CacheReady"
	AgentReady condition.Cond = "AgentReady"
	Stopped    condition.Cond = "Stopped"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=ippl;ippls,scope=Namespaced
// +kubebuilder:printcolumn:name="NETWORK",type=string,JSONPath=`.spec.networkName`
// +kubebuilder:printcolumn:name="AVAILABLE",type=integer,JSONPath=`.status.ipv4.available`
// +kubebuilder:printcolumn:name="USED",type=integer,JSONPath=`.status.ipv4.used`
// +kubebuilder:printcolumn:name="REGISTERED",type=string,JSONPath=`.status.conditions[?(@.type=='Registered')].status`
// +kubebuilder:printcolumn:name="CACHEREADY",type=string,JSONPath=`.status.conditions[?(@.type=='CacheReady')].status`
// +kubebuilder:printcolumn:name="AGENTREADY",type=string,JSONPath=`.status.conditions[?(@.type=='AgentReady')].status`
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

	// +optional
	Paused *bool `json:"paused,omitempty"`
}

type IPv4Config struct {
	ServerIP string `json:"serverIP,omitempty"`
	CIDR     string `json:"cidr,omitempty"`
	Pool     Pool   `json:"pool,omitempty"`

	// +optional
	Router string `json:"router,omitempty"`

	// +optional
	DNS []string `json:"dns,omitempty"`

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
	Start string `json:"start,omitempty"`
	End   string `json:"end,omitempty"`

	// +optional
	Exclude []string `json:"exclude,omitempty"`
}

type IPPoolStatus struct {
	LastUpdate metav1.Time `json:"lastUpdate,omitempty"`

	// +optional
	IPv4 *IPv4Status `json:"ipv4,omitempty"`

	// +optional
	AgentPodRef *PodReference `json:"agentPodRef,omitempty"`

	// +optional
	Conditions []genericcondition.GenericCondition `json:"conditions,omitempty"`
}

type IPv4Status struct {
	Allocated map[string]string `json:"allocated,omitempty"`
	Used      int               `json:"used"`
	Available int               `json:"available"`
}

type PodReference struct {
	Namespace string    `json:"namespace,omitempty"`
	Name      string    `json:"name,omitempty"`
	Image     string    `json:"image,omitempty"`
	UID       types.UID `json:"uid,omitempty"`
}
