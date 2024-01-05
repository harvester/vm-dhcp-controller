package ippool

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"

	"github.com/rancher/wrangler/pkg/kv"
	"github.com/rancher/wrangler/pkg/relatedresource"
	networkv1 "github.com/starbops/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/starbops/vm-dhcp-controller/pkg/config"
	ctlcorev1 "github.com/starbops/vm-dhcp-controller/pkg/generated/controllers/core/v1"
	ctlnetworkv1 "github.com/starbops/vm-dhcp-controller/pkg/generated/controllers/network.harvesterhci.io/v1alpha1"
	"github.com/starbops/vm-dhcp-controller/pkg/ipam"
)

const (
	controllerName = "vm-dhcp-ippool-controller"

	multusNetworksAnnotationKey = "k8s.v1.cni.cncf.io/networks"

	ipPoolNamespaceLabelKey = "network.harvesterhci.io/ippool-namespace"
	ipPoolNameLabelKey      = "network.harvesterhci.io/ippool-name"

	setIPAddrScript = `
#!/usr/bin/env sh
set -ex

ip address flush dev eth1
ip address add %s/%d dev eth1
`
)

var (
	runAsUserID  int64 = 0
	runAsGroupID int64 = 0
)

type Network struct {
	Namespace     string `json:"namespace"`
	Name          string `json:"name"`
	InterfaceName string `json:"interface"`
}

type Handler struct {
	agentNamespace          string
	agentImage              *config.Image
	agentServiceAccountName string
	noAgent                 bool
	noDHCP                  bool

	ipAllocator *ipam.IPAllocator

	ippoolController ctlnetworkv1.IPPoolController
	ippoolClient     ctlnetworkv1.IPPoolClient
	ippoolCache      ctlnetworkv1.IPPoolCache
	podClient        ctlcorev1.PodClient
	podCache         ctlcorev1.PodCache
}

func Register(ctx context.Context, management *config.Management) error {
	ippools := management.HarvesterNetworkFactory.Network().V1alpha1().IPPool()
	pods := management.CoreFactory.Core().V1().Pod()

	handler := &Handler{
		agentNamespace:          management.Options.AgentNamespace,
		agentImage:              management.Options.AgentImage,
		agentServiceAccountName: management.Options.AgentServiceAccountName,
		noAgent:                 management.Options.NoAgent,
		noDHCP:                  management.Options.NoDHCP,

		ipAllocator: management.IPAllocator,

		ippoolController: ippools,
		ippoolClient:     ippools,
		ippoolCache:      ippools.Cache(),
		podClient:        pods,
		podCache:         pods.Cache(),
	}

	ctlnetworkv1.RegisterIPPoolStatusHandler(
		ctx,
		ippools,
		networkv1.Registered,
		"ippool-register",
		handler.DeployAgent,
	)
	ctlnetworkv1.RegisterIPPoolStatusHandler(
		ctx,
		ippools,
		networkv1.Ready,
		"ippool-agent-monitor",
		handler.MonitorAgent,
	)

	relatedresource.Watch(ctx, "ippool-trigger", func(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
		var keys []relatedresource.Key
		sets := labels.Set{
			"network.harvesterhci.io/vm-dhcp-controller": "agent",
		}
		pods, err := handler.podCache.List(namespace, sets.AsSelector())
		if err != nil {
			return nil, err
		}
		for _, pod := range pods {
			key := relatedresource.Key{
				Namespace: pod.Labels[ipPoolNamespaceLabelKey],
				Name:      pod.Labels[ipPoolNameLabelKey],
			}
			keys = append(keys, key)
		}
		return keys, nil
	}, ippools, pods)

	ippools.OnChange(ctx, controllerName, handler.OnChange)
	ippools.OnRemove(ctx, controllerName, handler.OnRemove)

	return nil
}

func (h *Handler) OnChange(key string, ipPool *networkv1.IPPool) (*networkv1.IPPool, error) {
	if ipPool == nil || ipPool.DeletionTimestamp != nil {
		return nil, nil
	}

	klog.Infof("ippool configuration %s/%s has been changed: %+v", ipPool.Namespace, ipPool.Name, ipPool.Spec.IPv4Config)

	// Construct IPAM from IPPool spec

	klog.Infof("initialize ipam for ippool %s/%s", ipPool.Namespace, ipPool.Name)
	if err := h.ipAllocator.NewIPSubnet(
		ipPool.Spec.NetworkName,
		ipPool.Spec.IPv4Config.CIDR,
		ipPool.Spec.IPv4Config.Pool.Start,
		ipPool.Spec.IPv4Config.Pool.End,
	); err != nil {
		return nil, err
	}

	// Revoke excluded IP addresses in IPAM
	for _, ip := range ipPool.Spec.IPv4Config.Pool.Exclude {
		if err := h.ipAllocator.RevokeIP(ipPool.Spec.NetworkName, ip.String()); err != nil {
			return nil, err
		}
		klog.Infof("excluded ip %s was revoked in ipam %s", ip, ipPool.Spec.NetworkName)
	}

	// Construct IPAM from IPPool status

	if ipPool.Status.IPv4 != nil {
		for ip, mac := range ipPool.Status.IPv4.Allocated {
			if mac == ipam.ExcludedMark {
				continue
			}
			if _, err := h.ipAllocator.AllocateIP(ipPool.Spec.NetworkName, ip); err != nil {
				return nil, err
			}
			klog.Infof("previously allocated ip %s was re-allocated in ipam %s", ip, ipPool.Spec.NetworkName)
		}
	}

	klog.Infof("ipam %s for ippool %s/%s has been updated", ipPool.Spec.NetworkName, ipPool.Namespace, ipPool.Name)

	// Update IPPool status based on up-to-date IPAM

	ipPoolCpy := ipPool.DeepCopy()

	ipv4Status := ipPoolCpy.Status.IPv4
	if ipv4Status == nil {
		ipv4Status = new(networkv1.IPv4Status)
	}

	used, err := h.ipAllocator.GetUsed(ipPool.Spec.NetworkName)
	if err != nil {
		return nil, err
	}
	ipv4Status.Used = used

	available, err := h.ipAllocator.GetAvailable(ipPool.Spec.NetworkName)
	if err != nil {
		return nil, err
	}
	ipv4Status.Available = available

	allocated := ipv4Status.Allocated
	if allocated == nil {
		allocated = make(map[string]string)
	}
	for _, v := range ipPool.Spec.IPv4Config.Pool.Exclude {
		allocated[v.String()] = ipam.ExcludedMark
	}
	ipv4Status.Allocated = allocated

	ipPoolCpy.Status.IPv4 = ipv4Status

	if !reflect.DeepEqual(ipPoolCpy, ipPool) {
		klog.Infof("update ippool %s/%s", ipPool.Namespace, ipPool.Name)
		ipPoolCpy.Status.LastUpdate = metav1.Now()
		return h.ippoolClient.UpdateStatus(ipPoolCpy)
	}

	return ipPool, nil
}

func (h *Handler) OnRemove(key string, ipPool *networkv1.IPPool) (*networkv1.IPPool, error) {
	if ipPool == nil {
		return nil, nil
	}

	klog.Infof("ippool configuration %s/%s has been removed", ipPool.Namespace, ipPool.Name)

	if h.noAgent {
		return ipPool, nil
	}

	if ipPool.Status.AgentPodRef == nil {
		return ipPool, nil
	}

	klog.Infof("remove the backing agent %s/%s for ippool %s/%s", ipPool.Status.AgentPodRef.Namespace, ipPool.Status.AgentPodRef.Name, ipPool.Namespace, ipPool.Name)
	if err := h.podClient.Delete(ipPool.Status.AgentPodRef.Namespace, ipPool.Status.AgentPodRef.Name, &metav1.DeleteOptions{}); err != nil {
		return nil, err
	}

	return ipPool, nil
}

func (h *Handler) DeployAgent(ipPool *networkv1.IPPool, status networkv1.IPPoolStatus) (networkv1.IPPoolStatus, error) {
	klog.Infof("deploy agent for ippool %s/%s", ipPool.Namespace, ipPool.Name)

	if h.noAgent {
		return status, nil
	}

	agent, err := h.prepareAgentPod(ipPool)
	if err != nil {
		return status, err
	}

	agentPod, err := h.podClient.Create(agent)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return status, nil
		}
		return status, err
	}

	klog.Infof("agent for ippool %s/%s has been deployed", ipPool.Namespace, ipPool.Name)

	status.AgentPodRef = &networkv1.PodReference{
		Namespace: agentPod.Namespace,
		Name:      agentPod.Name,
	}

	return status, nil
}

func (h *Handler) MonitorAgent(ipPool *networkv1.IPPool, status networkv1.IPPoolStatus) (networkv1.IPPoolStatus, error) {
	klog.Infof("monitor agent for ippool %s/%s", ipPool.Namespace, ipPool.Name)

	if ipPool.Status.AgentPodRef == nil {
		return status, fmt.Errorf("agent for ippool %s/%s is not deployed", ipPool.Namespace, ipPool.Name)
	}

	agentPod, err := h.podCache.Get(ipPool.Status.AgentPodRef.Namespace, ipPool.Status.AgentPodRef.Name)
	if err != nil {
		return status, err
	}

	if !isPodReady(agentPod) {
		return status, fmt.Errorf("agent for ippool %s/%s is not ready", agentPod.Namespace, agentPod.Name)
	}

	return status, nil
}

func (h *Handler) prepareAgentPod(ipPool *networkv1.IPPool) (*corev1.Pod, error) {
	name := fmt.Sprintf("%s-%s-agent", ipPool.Namespace, ipPool.Name)

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
	if h.noDHCP {
		args = append(args, "--dry-run")
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				multusNetworksAnnotationKey: string(networksStr),
			},
			Labels: map[string]string{
				"network.harvesterhci.io/vm-dhcp-controller": "agent",
				ipPoolNamespaceLabelKey:                      ipPool.Namespace,
				ipPoolNameLabelKey:                           ipPool.Name,
			},
			Name:      name,
			Namespace: h.agentNamespace,
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: h.agentServiceAccountName,
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
					Image: h.agentImage.String(),
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
	}, nil
}

func isPodReady(pod *corev1.Pod) bool {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}
