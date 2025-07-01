package v1alpha1

import (
	"github.com/rancher/wrangler/pkg/condition"
	"github.com/rancher/wrangler/pkg/genericcondition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	Registered condition.Cond = "Registered"
	CacheReady condition.Cond = "CacheReady"
	Stopped    condition.Cond = "Stopped"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=ippl;ippls,scope=Namespaced
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="NETWORK",type=string,JSONPath=`.spec.networkName`
// +kubebuilder:printcolumn:name="AVAILABLE",type=integer,JSONPath=`.status.ipv4.available`
// +kubebuilder:printcolumn:name="USED",type=integer,JSONPath=`.status.ipv4.used`
// +kubebuilder:printcolumn:name="REGISTERED",type=string,JSONPath=`.status.conditions[?(@.type=='Registered')].status`
// +kubebuilder:printcolumn:name="CACHEREADY",type=string,JSONPath=`.status.conditions[?(@.type=='CacheReady')].status`
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=`.metadata.creationTimestamp`

type IPPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IPPoolSpec   `json:"spec,omitempty"`
	Status IPPoolStatus `json:"status,omitempty"`
}

type IPPoolSpec struct {
	IPv4Config IPv4Config `json:"ipv4Config,omitempty"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="NetworkName is immutable"
	// +kubebuilder:validation:MaxLength=64
	NetworkName string `json:"networkName"`

	// +optional
	// +kubebuilder:validation:Optional
	Paused *bool `json:"paused,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="!has(oldSelf.router) || has(self.router)", message="Router is required once set"
type IPv4Config struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="CIDR is immutable"
	CIDR string `json:"cidr"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Format=ipv4
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="ServerIP is immutable"
	ServerIP string `json:"serverIP"`

	// +kubebuilder:validation:Required
	Pool Pool `json:"pool"`

	// +optional
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Format=ipv4
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Router is immutable"
	Router string `json:"router,omitempty"`

	// +optional
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Format=ipv4
	// +kubebuilder:validation:MaxItems=3
	DNS []string `json:"dns,omitempty"`

	// +optional
	// +kubebuilder:validation:Optional
	DomainName *string `json:"domainName,omitempty"`

	// +optional
	// +kubebuilder:validation:Optional
	DomainSearch []string `json:"domainSearch,omitempty"`

	// +optional
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxItems=4
	NTP []string `json:"ntp,omitempty"`

	// +optional
	// +kubebuilder:validation:Optional
	LeaseTime *int `json:"leaseTime,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="!has(oldSelf.exclude) || has(self.exclude)", message="End is required once set"
type Pool struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Format=ipv4
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Start is immutable"
	Start string `json:"start"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Format=ipv4
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="End is immutable"
	End string `json:"end"`

	// +optional
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Format=ipv4
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Exclude is immutable"
	Exclude []string `json:"exclude,omitempty"`
}

type IPPoolStatus struct {
	LastUpdate metav1.Time `json:"lastUpdate,omitempty"`

	// +optional
	// +kubebuilder:validation:Optional
	IPv4 *IPv4Status `json:"ipv4,omitempty"`

	// +optional
	// +kubebuilder:validation:Optional
	Conditions []genericcondition.GenericCondition `json:"conditions,omitempty"`

	// +optional
	// +kubebuilder:validation:Optional
	AgentPodRef *PodReference `json:"agentPodRef,omitempty"`
}

type IPv4Status struct {
	Allocated map[string]string `json:"allocated,omitempty"`
	Used      int               `json:"used"`
	Available int               `json:"available"`
}

// PodReference contains enough information to locate the referenced pod.
type PodReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	UID       string `json:"uid"`
}

