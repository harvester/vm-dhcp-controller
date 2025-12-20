package ippool

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	cniv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/rancher/wrangler/v3/pkg/kv"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io"
	networkv1 "github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/harvester/vm-dhcp-controller/pkg/util"
)

func prepareAgentDeployment(
	ipPool *networkv1.IPPool,
	noDHCP bool,
	agentNamespace string,
	clusterNetwork string,
	agentServiceAccountName string,
	agentImage string,
) (*appsv1.Deployment, error) {
	name := util.SafeAgentConcatName(ipPool.Namespace, ipPool.Name)

	nadNamespace, nadName := kv.RSplit(ipPool.Spec.NetworkName, "/")
	networks := []Network{
		{
			Namespace:     nadNamespace,
			Name:          nadName,
			InterfaceName: "eth1",
		},
	}
	networksStr, err := json.Marshal(networks)
	if err != nil {
		return nil, err
	}

	_, ipNet, err := net.ParseCIDR(ipPool.Spec.IPv4Config.CIDR)
	if err != nil {
		return nil, err
	}
	prefixLength, _ := ipNet.Mask.Size()

	args := []string{
		"--ippool-ref",
		fmt.Sprintf("%s/%s", ipPool.Namespace, ipPool.Name),
	}
	if noDHCP {
		args = append(args, "--dry-run")
	}

	labels := map[string]string{
		vmDHCPControllerLabelKey:     "agent",
		util.IPPoolNamespaceLabelKey: ipPool.Namespace,
		util.IPPoolNameLabelKey:      ipPool.Name,
	}
	replicas := int32(1)
	maxUnavailable := intstr.FromInt(0)
	maxSurge := intstr.FromInt(1)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels:    labels,
			Name:      name,
			Namespace: agentNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: &maxUnavailable,
					MaxSurge:       &maxSurge,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						multusNetworksAnnotationKey: string(networksStr),
					},
					Labels: labels,
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
							Name:                     "ip-setter",
							Image:                    agentImage,
							ImagePullPolicy:          corev1.PullIfNotPresent,
							TerminationMessagePath:   corev1.TerminationMessagePathDefault,
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
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
							Name:                     "agent",
							Image:                    agentImage,
							ImagePullPolicy:          corev1.PullIfNotPresent,
							TerminationMessagePath:   corev1.TerminationMessagePathDefault,
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
							Args:                     args,
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
			},
		},
	}, nil
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

func (b *IPPoolBuilder) Annotation(key, value string) *IPPoolBuilder {
	if b.ipPool.Annotations == nil {
		b.ipPool.Annotations = make(map[string]string)
	}
	b.ipPool.Annotations[key] = value
	return b
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

func (b *IPPoolBuilder) Router(router string) *IPPoolBuilder {
	b.ipPool.Spec.IPv4Config.Router = router
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

func (b *IPPoolBuilder) AgentDeploymentRef(namespace, name, image, uid string) *IPPoolBuilder {
	if b.ipPool.Status.AgentDeploymentRef == nil {
		b.ipPool.Status.AgentDeploymentRef = new(networkv1.DeploymentReference)
	}
	b.ipPool.Status.AgentDeploymentRef.Namespace = namespace
	b.ipPool.Status.AgentDeploymentRef.Name = name
	b.ipPool.Status.AgentDeploymentRef.Image = image
	b.ipPool.Status.AgentDeploymentRef.UID = types.UID(uid)
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

func (b *ipPoolStatusBuilder) AgentDeploymentRef(namespace, name, image, uid string) *ipPoolStatusBuilder {
	if b.ipPoolStatus.AgentDeploymentRef == nil {
		b.ipPoolStatus.AgentDeploymentRef = new(networkv1.DeploymentReference)
	}
	b.ipPoolStatus.AgentDeploymentRef.Namespace = namespace
	b.ipPoolStatus.AgentDeploymentRef.Name = name
	b.ipPoolStatus.AgentDeploymentRef.Image = image
	b.ipPoolStatus.AgentDeploymentRef.UID = types.UID(uid)
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

type deploymentBuilder struct {
	deployment *appsv1.Deployment
}

func newDeploymentBuilder(namespace, name string) *deploymentBuilder {
	replicas := int32(1)
	return &deploymentBuilder{
		deployment: &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
				Template: corev1.PodTemplateSpec{},
			},
		},
	}
}

func (b *deploymentBuilder) Container(name, repository, tag string) *deploymentBuilder {
	container := corev1.Container{
		Name:  name,
		Image: repository + ":" + tag,
	}
	b.deployment.Spec.Template.Spec.Containers = append(b.deployment.Spec.Template.Spec.Containers, container)
	return b
}

func (b *deploymentBuilder) DeploymentReady(ready bool) *deploymentBuilder {
	if !ready {
		return b
	}
	if b.deployment.Generation == 0 {
		b.deployment.Generation = 1
	}
	desired := int32(1)
	if b.deployment.Spec.Replicas != nil {
		desired = *b.deployment.Spec.Replicas
	}
	b.deployment.Status.UpdatedReplicas = desired
	b.deployment.Status.AvailableReplicas = desired
	b.deployment.Status.ReadyReplicas = desired
	b.deployment.Status.ObservedGeneration = b.deployment.Generation
	return b
}

func (b *deploymentBuilder) Build() *appsv1.Deployment {
	return b.deployment
}

type NetworkAttachmentDefinitionBuilder struct {
	nad *cniv1.NetworkAttachmentDefinition
}

func NewNetworkAttachmentDefinitionBuilder(namespace, name string) *NetworkAttachmentDefinitionBuilder {
	return &NetworkAttachmentDefinitionBuilder{
		nad: &cniv1.NetworkAttachmentDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
		},
	}
}

func (b *NetworkAttachmentDefinitionBuilder) Label(key, value string) *NetworkAttachmentDefinitionBuilder {
	if b.nad.Labels == nil {
		b.nad.Labels = make(map[string]string)
	}
	b.nad.Labels[key] = value
	return b
}

func (b *NetworkAttachmentDefinitionBuilder) Build() *cniv1.NetworkAttachmentDefinition {
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
