package ippool

import (
	"encoding/json"
	"fmt"
	"net"

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

func setDisabledCondition(ipPool *networkv1.IPPool, status corev1.ConditionStatus, reason, message string) {
	networkv1.Disabled.SetStatus(ipPool, string(status))
	networkv1.Disabled.Reason(ipPool, reason)
	networkv1.Disabled.Message(ipPool, message)
}

type ipPoolBuilder struct {
	ipPool *networkv1.IPPool
}

func newIPPoolBuilder(namespace, name string) *ipPoolBuilder {
	return &ipPoolBuilder{
		ipPool: &networkv1.IPPool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		},
	}
}

func (b *ipPoolBuilder) NetworkName(networkName string) *ipPoolBuilder {
	b.ipPool.Spec.NetworkName = networkName
	return b
}

func (b *ipPoolBuilder) Paused() *ipPoolBuilder {
	paused := true
	b.ipPool.Spec.Paused = &paused
	return b
}

func (b *ipPoolBuilder) UnPaused() *ipPoolBuilder {
	paused := false
	b.ipPool.Spec.Paused = &paused
	return b
}

func (b *ipPoolBuilder) ServerIP(serverIP string) *ipPoolBuilder {
	b.ipPool.Spec.IPv4Config.ServerIP = serverIP
	return b
}

func (b *ipPoolBuilder) CIDR(cidr string) *ipPoolBuilder {
	b.ipPool.Spec.IPv4Config.CIDR = cidr
	return b
}

func (b *ipPoolBuilder) PoolRange(start, end string) *ipPoolBuilder {
	b.ipPool.Spec.IPv4Config.Pool.Start = start
	b.ipPool.Spec.IPv4Config.Pool.End = end
	return b
}

func (b *ipPoolBuilder) Exclude(exclude []string) *ipPoolBuilder {
	b.ipPool.Spec.IPv4Config.Pool.Exclude = exclude
	return b
}

func (b *ipPoolBuilder) AgentPodRef(namespace, name string) *ipPoolBuilder {
	if b.ipPool.Status.AgentPodRef == nil {
		b.ipPool.Status.AgentPodRef = new(networkv1.PodReference)
	}
	b.ipPool.Status.AgentPodRef.Namespace = namespace
	b.ipPool.Status.AgentPodRef.Name = name
	return b
}

func (b *ipPoolBuilder) RegisteredCondition(status corev1.ConditionStatus, reason, message string) *ipPoolBuilder {
	setRegisteredCondition(b.ipPool, status, reason, message)
	return b
}

func (b *ipPoolBuilder) CacheReadyCondition(status corev1.ConditionStatus, reason, message string) *ipPoolBuilder {
	setCacheReadyCondition(b.ipPool, status, reason, message)
	return b
}

func (b *ipPoolBuilder) AgentReadyCondition(status corev1.ConditionStatus, reason, message string) *ipPoolBuilder {
	setAgentReadyCondition(b.ipPool, status, reason, message)
	return b
}

func (b *ipPoolBuilder) DisabledCondition(status corev1.ConditionStatus, reason, message string) *ipPoolBuilder {
	setDisabledCondition(b.ipPool, status, reason, message)
	return b
}

func (p *ipPoolBuilder) Build() *networkv1.IPPool {
	return p.ipPool
}
