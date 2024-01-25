package ippool

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	cniv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/rancher/wrangler/pkg/kv"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io"
	networkv1 "github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/harvester/vm-dhcp-controller/pkg/config"
)

func prepareAgentPod(
	ipPool *networkv1.IPPool,
	noDHCP bool,
	agentNamespace string,
	clusterNetwork string,
	agentServiceAccountName string,
	agentImage *config.Image,
) *corev1.Pod {
	name := fmt.Sprintf("%s-%s-agent", ipPool.Namespace, ipPool.Name)

	nadNamespace, nadName := kv.RSplit(ipPool.Spec.NetworkName, "/")
	networks := []Network{
		{
			Namespace:     nadNamespace,
			Name:          nadName,
			InterfaceName: "eth1",
		},
	}
	networksStr, _ := json.Marshal(networks)

	_, ipNet, _ := net.ParseCIDR(ipPool.Spec.IPv4Config.CIDR)
	prefixLength, _ := ipNet.Mask.Size()

	args := []string{
		"--ippool-ref",
		fmt.Sprintf("%s/%s", ipPool.Namespace, ipPool.Name),
	}
	if noDHCP {
		args = append(args, "--dry-run")
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				multusNetworksAnnotationKey: string(networksStr),
			},
			Labels: map[string]string{
				vmDHCPControllerLabelKey: "agent",
				ipPoolNamespaceLabelKey:  ipPool.Namespace,
				ipPoolNameLabelKey:       ipPool.Name,
			},
			Name:      name,
			Namespace: agentNamespace,
		},
		Spec: corev1.PodSpec{
			Affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      network.GroupName + "/" + clusterNetwork,
										Operator: corev1.NodeSelectorOpIn,
										Values: []string{
											"true",
										},
									},
								},
							},
						},
					},
				},
			},
			ServiceAccountName: agentServiceAccountName,
			InitContainers: []corev1.Container{
				{
					Name:  "ip-setter",
					Image: "busybox",
					Command: []string{
						"/bin/sh",
						"-c",
						fmt.Sprintf(setIPAddrScript, ipPool.Spec.IPv4Config.ServerIP, prefixLength),
					},
					SecurityContext: &corev1.SecurityContext{
						RunAsUser:  &runAsUserID,
						RunAsGroup: &runAsGroupID,
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{
								"NET_ADMIN",
							},
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:  "agent",
					Image: agentImage.String(),
					Args:  args,
					Env: []corev1.EnvVar{
						{
							Name:  "VM_DHCP_AGENT_NAME",
							Value: name,
						},
					},
					SecurityContext: &corev1.SecurityContext{
						RunAsUser:  &runAsUserID,
						RunAsGroup: &runAsGroupID,
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{
								"NET_ADMIN",
							},
						},
					},
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/healthz",
								Port: intstr.FromInt(8080),
							},
						},
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/readyz",
								Port: intstr.FromInt(8080),
							},
						},
					},
				},
			},
		},
	}
}

func setRegisteredCondition(ipPool *networkv1.IPPool, status corev1.ConditionStatus, reason, message string) {
	networkv1.Registered.SetStatus(ipPool, string(status))
	networkv1.Registered.Reason(ipPool, reason)
	networkv1.Registered.Message(ipPool, message)
}

func setCacheReadyCondition(ipPool *networkv1.IPPool, status corev1.ConditionStatus, reason, message string) {
	networkv1.CacheReady.SetStatus(ipPool, string(status))
	networkv1.CacheReady.Reason(ipPool, reason)
	networkv1.CacheReady.Message(ipPool, message)
}

func setAgentReadyCondition(ipPool *networkv1.IPPool, status corev1.ConditionStatus, reason, message string) {
	networkv1.AgentReady.SetStatus(ipPool, string(status))
	networkv1.AgentReady.Reason(ipPool, reason)
	networkv1.AgentReady.Message(ipPool, message)
}

func setStoppedCondition(ipPool *networkv1.IPPool, status corev1.ConditionStatus, reason, message string) {
	networkv1.Stopped.SetStatus(ipPool, string(status))
	networkv1.Stopped.Reason(ipPool, reason)
	networkv1.Stopped.Message(ipPool, message)
}

type IPPoolBuilder struct {
	ipPool *networkv1.IPPool
}

func NewIPPoolBuilder(namespace, name string) *IPPoolBuilder {
	return &IPPoolBuilder{
		ipPool: &networkv1.IPPool{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
		},
	}
}

func (b *IPPoolBuilder) NetworkName(networkName string) *IPPoolBuilder {
	b.ipPool.Spec.NetworkName = networkName
	return b
}

func (b *IPPoolBuilder) Paused() *IPPoolBuilder {
	paused := true
	b.ipPool.Spec.Paused = &paused
	return b
}

func (b *IPPoolBuilder) UnPaused() *IPPoolBuilder {
	paused := false
	b.ipPool.Spec.Paused = &paused
	return b
}

func (b *IPPoolBuilder) ServerIP(serverIP string) *IPPoolBuilder {
	b.ipPool.Spec.IPv4Config.ServerIP = serverIP
	return b
}

func (b *IPPoolBuilder) CIDR(cidr string) *IPPoolBuilder {
	b.ipPool.Spec.IPv4Config.CIDR = cidr
	return b
}

func (b *IPPoolBuilder) PoolRange(start, end string) *IPPoolBuilder {
	b.ipPool.Spec.IPv4Config.Pool.Start = start
	b.ipPool.Spec.IPv4Config.Pool.End = end
	return b
}

func (b *IPPoolBuilder) Exclude(ipAddressList ...string) *IPPoolBuilder {
	b.ipPool.Spec.IPv4Config.Pool.Exclude = append(b.ipPool.Spec.IPv4Config.Pool.Exclude, ipAddressList...)
	return b
}

func (b *IPPoolBuilder) AgentPodRef(namespace, name string) *IPPoolBuilder {
	if b.ipPool.Status.AgentPodRef == nil {
		b.ipPool.Status.AgentPodRef = new(networkv1.PodReference)
	}
	b.ipPool.Status.AgentPodRef.Namespace = namespace
	b.ipPool.Status.AgentPodRef.Name = name
	return b
}

func (b *IPPoolBuilder) Allocated(ipAddress, macAddress string) *IPPoolBuilder {
	if b.ipPool.Status.IPv4 == nil {
		b.ipPool.Status.IPv4 = new(networkv1.IPv4Status)
	}
	if b.ipPool.Status.IPv4.Allocated == nil {
		b.ipPool.Status.IPv4.Allocated = make(map[string]string, 2)
	}
	b.ipPool.Status.IPv4.Allocated[ipAddress] = macAddress
	return b
}

func (b *IPPoolBuilder) Available(count int) *IPPoolBuilder {
	if b.ipPool.Status.IPv4 == nil {
		b.ipPool.Status.IPv4 = new(networkv1.IPv4Status)
	}
	b.ipPool.Status.IPv4.Available = count
	return b
}

func (b *IPPoolBuilder) Used(count int) *IPPoolBuilder {
	if b.ipPool.Status.IPv4 == nil {
		b.ipPool.Status.IPv4 = new(networkv1.IPv4Status)
	}
	b.ipPool.Status.IPv4.Used = count
	return b
}

func (b *IPPoolBuilder) RegisteredCondition(status corev1.ConditionStatus, reason, message string) *IPPoolBuilder {
	setRegisteredCondition(b.ipPool, status, reason, message)
	return b
}

func (b *IPPoolBuilder) CacheReadyCondition(status corev1.ConditionStatus, reason, message string) *IPPoolBuilder {
	setCacheReadyCondition(b.ipPool, status, reason, message)
	return b
}

func (b *IPPoolBuilder) AgentReadyCondition(status corev1.ConditionStatus, reason, message string) *IPPoolBuilder {
	setAgentReadyCondition(b.ipPool, status, reason, message)
	return b
}

func (b *IPPoolBuilder) StoppedCondition(status corev1.ConditionStatus, reason, message string) *IPPoolBuilder {
	setStoppedCondition(b.ipPool, status, reason, message)
	return b
}

func (b *IPPoolBuilder) Build() *networkv1.IPPool {
	return b.ipPool
}

type ipPoolStatusBuilder struct {
	ipPoolStatus networkv1.IPPoolStatus
}

func newIPPoolStatusBuilder() *ipPoolStatusBuilder {
	return &ipPoolStatusBuilder{
		ipPoolStatus: networkv1.IPPoolStatus{},
	}
}

func (b *ipPoolStatusBuilder) AgentPodRef(namespace, name string) *ipPoolStatusBuilder {
	if b.ipPoolStatus.AgentPodRef == nil {
		b.ipPoolStatus.AgentPodRef = new(networkv1.PodReference)
	}
	b.ipPoolStatus.AgentPodRef.Namespace = namespace
	b.ipPoolStatus.AgentPodRef.Name = name
	return b
}

func (b *ipPoolStatusBuilder) RegisteredCondition(status corev1.ConditionStatus, reason, message string) *ipPoolStatusBuilder {
	networkv1.Registered.SetStatus(&b.ipPoolStatus, string(status))
	networkv1.Registered.Reason(&b.ipPoolStatus, reason)
	networkv1.Registered.Message(&b.ipPoolStatus, message)
	return b
}

func (b *ipPoolStatusBuilder) CacheReadyCondition(status corev1.ConditionStatus, reason, message string) *ipPoolStatusBuilder {
	networkv1.CacheReady.SetStatus(&b.ipPoolStatus, string(status))
	networkv1.CacheReady.Reason(&b.ipPoolStatus, reason)
	networkv1.CacheReady.Message(&b.ipPoolStatus, message)
	return b
}

func (b *ipPoolStatusBuilder) AgentReadyCondition(status corev1.ConditionStatus, reason, message string) *ipPoolStatusBuilder {
	networkv1.AgentReady.SetStatus(&b.ipPoolStatus, string(status))
	networkv1.AgentReady.Reason(&b.ipPoolStatus, reason)
	networkv1.AgentReady.Message(&b.ipPoolStatus, message)
	return b
}

func (b *ipPoolStatusBuilder) StoppedCondition(status corev1.ConditionStatus, reason, message string) *ipPoolStatusBuilder {
	networkv1.Stopped.SetStatus(&b.ipPoolStatus, string(status))
	networkv1.Stopped.Reason(&b.ipPoolStatus, reason)
	networkv1.Stopped.Message(&b.ipPoolStatus, message)
	return b
}

func (b *ipPoolStatusBuilder) Build() networkv1.IPPoolStatus {
	return b.ipPoolStatus
}

type podBuilder struct {
	pod *corev1.Pod
}

func newPodBuilder(namespace, name string) *podBuilder {
	return &podBuilder{
		pod: &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
		},
	}
}

func (b *podBuilder) PodReady(ready corev1.ConditionStatus) *podBuilder {
	var found bool
	if b.pod.Status.Conditions == nil {
		b.pod.Status.Conditions = make([]corev1.PodCondition, 0, 1)
	}
	for i := range b.pod.Status.Conditions {
		if b.pod.Status.Conditions[i].Type == corev1.PodReady {
			b.pod.Status.Conditions[i].Status = corev1.ConditionTrue
			break
		}
	}
	if !found {
		b.pod.Status.Conditions = append(b.pod.Status.Conditions, corev1.PodCondition{
			Type:   corev1.PodReady,
			Status: corev1.ConditionTrue,
		})
	}
	return b
}

func (b *podBuilder) Build() *corev1.Pod {
	return b.pod
}

type networkAttachmentDefinitionBuilder struct {
	nad *cniv1.NetworkAttachmentDefinition
}

func newNetworkAttachmentDefinitionBuilder(namespace, name string) *networkAttachmentDefinitionBuilder {
	return &networkAttachmentDefinitionBuilder{
		nad: &cniv1.NetworkAttachmentDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
		},
	}
}

func (b *networkAttachmentDefinitionBuilder) Label(key, value string) *networkAttachmentDefinitionBuilder {
	if b.nad.Labels == nil {
		b.nad.Labels = make(map[string]string)
	}
	b.nad.Labels[key] = value
	return b
}

func (b *networkAttachmentDefinitionBuilder) Build() *cniv1.NetworkAttachmentDefinition {
	return b.nad
}

func SanitizeStatus(status *networkv1.IPPoolStatus) {
	now := time.Time{}
	status.LastUpdate = metav1.NewTime(now)
	for i := range status.Conditions {
		status.Conditions[i].LastTransitionTime = ""
		status.Conditions[i].LastUpdateTime = ""
	}
}
