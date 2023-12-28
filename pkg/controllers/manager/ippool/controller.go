package ippool

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	networkv1 "github.com/starbops/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/starbops/vm-dhcp-controller/pkg/config"
	"github.com/starbops/vm-dhcp-controller/pkg/controllers/agent/ippool"
	ctlcorev1 "github.com/starbops/vm-dhcp-controller/pkg/generated/controllers/core/v1"
	ctlnetworkv1 "github.com/starbops/vm-dhcp-controller/pkg/generated/controllers/network.harvesterhci.io/v1alpha1"
)

const (
	controllerName = "vm-dhcp-ippool-controller"

	multusNetworksAnnotationKey = "k8s.v1.cni.cncf.io/networks"

	ipPoolNamespaceLabelKey = "network.harvesterhci.io/ippool-namespace"
	ipPoolNameLabelKey      = "network.harvesterhci.io/ippool-name"

	agentCheckInterval = 5 * time.Second

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

		ippoolController: ippools,
		ippoolClient:     ippools,
		ippoolCache:      ippools.Cache(),
		podClient:        pods,
		podCache:         pods.Cache(),
	}

	ippools.OnChange(ctx, controllerName, handler.OnChange)
	ippools.OnRemove(ctx, controllerName, handler.OnRemove)

	return nil
}

func (h *Handler) OnChange(key string, ipPool *networkv1.IPPool) (*networkv1.IPPool, error) {
	if ipPool == nil || ipPool.DeletionTimestamp != nil {
		return nil, nil
	}

	klog.Infof("ippool configuration %s/%s has been changed: %+v", ipPool.Namespace, ipPool.Name, ipPool.Spec.IPv4Config)

	ipPoolCpy := ipPool.DeepCopy()

	// Register newly created IPPool object:
	// - Create a backing agent Pod
	// - Construct a in-memory IPAM module
	if networkv1.Registered.GetStatus(ipPool) == "" {
		klog.Infof("ippool %s/%s has no agent found", ipPool.Namespace, ipPool.Name)

		// Create the backing agent Pod
		agent, err := h.prepareAgentPod(ipPool)
		if err != nil {
			return ipPool, err
		}
		agentPod, err := h.podClient.Create(agent)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return ipPool, err
		}

		klog.Infof("agent pod %s/%s for backing %s/%s ippool has been created", agentPod.Namespace, agentPod.Name, ipPool.Namespace, ipPool.Name)

		// Update IPPool status
		ipPoolCpy.Status.LastUpdate = metav1.Now()

		// Agent Pod status
		agentPodRef := networkv1.PodReference{
			Namespace: agentPod.Namespace,
			Name:      agentPod.Name,
		}
		ipPoolCpy.Status.AgentPodRef = agentPodRef

		networkv1.Registered.SetStatus(ipPoolCpy, string(corev1.ConditionTrue))
		networkv1.Registered.Reason(ipPoolCpy, "")
		networkv1.Registered.Message(ipPoolCpy, "")

		if !reflect.DeepEqual(ipPoolCpy, ipPool) {
			klog.Infof("update ippool %s/%s", ipPool.Namespace, ipPool.Name)
			return h.ippoolClient.UpdateStatus(ipPoolCpy)
		}

		return ipPool, nil
	}

	// Monitor the backing agent Pod for the registered IPPool object:
	// - Each IPPool object should have one and only one agent serves as its backend for the data plane
	// - When the agent Pod is ready, mark the IPPool object as ready
	if networkv1.Ready.GetStatus(ipPool) == "" {
		sets := labels.Set{
			ipPoolNamespaceLabelKey: ipPool.Namespace,
			ipPoolNameLabelKey:      ipPool.Name,
		}
		pods, err := h.podCache.List(h.agentNamespace, sets.AsSelector())
		if err != nil {
			return ipPool, err
		}

		// Wait for the agent Pod being created...
		if len(pods) == 0 {
			h.ippoolController.EnqueueAfter(ipPool.Namespace, ipPool.Name, agentCheckInterval)
			return ipPool, nil
		}

		// There should not be more than one agent Pod running...
		if len(pods) > 1 {
			networkv1.Ready.SetStatus(ipPoolCpy, string(corev1.ConditionFalse))
			networkv1.Ready.Reason(ipPoolCpy, "MultiAgentPresented")
			networkv1.Ready.Message(ipPoolCpy, fmt.Sprintf("There are %d agent pods running", len(pods)))
			return h.ippoolClient.UpdateStatus(ipPoolCpy)
		}

		agentPod := pods[0]
		if !isPodReady(agentPod) {
			h.ippoolController.EnqueueAfter(ipPool.Namespace, ipPool.Name, agentCheckInterval)
			return ipPool, nil
		}

		networkv1.Ready.SetStatus(ipPoolCpy, string(corev1.ConditionTrue))
		networkv1.Ready.Reason(ipPoolCpy, "AgentRunning")
		networkv1.Ready.Message(ipPoolCpy, fmt.Sprintf("Agent Pod %s/%s is serving", agentPod.Namespace, agentPod.Name))
		return h.ippoolClient.UpdateStatus(ipPoolCpy)
	}

	return ipPool, nil
}

func (h *Handler) OnRemove(key string, ipPool *networkv1.IPPool) (*networkv1.IPPool, error) {
	if ipPool == nil {
		return nil, nil
	}

	klog.Infof("ippool configuration %s/%s has been removed", ipPool.Namespace, ipPool.Name)

	if networkv1.Ready.IsTrue(ipPool) {
		klog.Infof("remove the backing agent %s/%s for ippool %s/%s", ipPool.Status.AgentPodRef.Namespace, ipPool.Status.AgentPodRef.Name, ipPool.Namespace, ipPool.Name)
		if err := h.podClient.Delete(ipPool.Status.AgentPodRef.Namespace, ipPool.Status.AgentPodRef.Name, &metav1.DeleteOptions{}); err != nil {
			return ipPool, err
		}

		return ipPool, nil
	}

	return ipPool, nil
}

func (h *Handler) prepareAgentPod(ipPool *networkv1.IPPool) (*corev1.Pod, error) {
	name := fmt.Sprintf("%s-%s-agent", ipPool.Namespace, ipPool.Name)

	networks := []Network{
		{
			Namespace:     ipPool.Namespace,
			Name:          ipPool.Spec.NetworkName,
			InterfaceName: "eth1",
		},
	}
	networksStr, err := json.Marshal(networks)
	if err != nil {
		return nil, err
	}

	_, mask, err := net.ParseCIDR(ipPool.Spec.IPv4Config.CIDR)
	if err != nil {
		return nil, err
	}
	prefixLength, _ := mask.Mask.Size()

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				multusNetworksAnnotationKey: string(networksStr),
			},
			Labels: map[string]string{
				ipPoolNamespaceLabelKey: ipPool.Namespace,
				ipPoolNameLabelKey:      ipPool.Name,
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
					Name:  "vdca",
					Image: h.agentImage.String(),
					Args: []string{
						"agent",
						"--name",
						fmt.Sprintf("%s-%s-agent", ipPool.Namespace, ipPool.Name),
						"--pool-namespace",
						ipPool.Namespace,
						"--pool-name",
						ipPool.Name,
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
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							Exec: &corev1.ExecAction{
								Command: []string{
									"cat",
									ippool.PIDFilePath,
								},
							},
						},
						InitialDelaySeconds: 5,
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
