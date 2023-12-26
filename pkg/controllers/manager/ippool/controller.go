package ippool

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	networkv1 "github.com/starbops/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/starbops/vm-dhcp-controller/pkg/config"
	ctlcorev1 "github.com/starbops/vm-dhcp-controller/pkg/generated/controllers/core/v1"
	ctlnetworkv1 "github.com/starbops/vm-dhcp-controller/pkg/generated/controllers/network.harvesterhci.io/v1alpha1"
)

const (
	controllerName = "vm-dhcp-ippool-controller"

	multusNetworksAnnotationKey = "k8s.v1.cni.cncf.io/networks"

	ipPoolLabelKey = "network.harvesterhci.io/ippool"

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
	agentImage *config.Image

	ippoolClient ctlnetworkv1.IPPoolClient
	ippoolCache  ctlnetworkv1.IPPoolCache
	podClient    ctlcorev1.PodClient
	podCache     ctlcorev1.PodCache
}

func Register(ctx context.Context, management *config.Management) error {
	ippools := management.HarvesterNetworkFactory.Network().V1alpha1().IPPool()
	pods := management.CoreFactory.Core().V1().Pod()

	handler := &Handler{
		agentImage: management.Options.AgentImage,

		ippoolClient: ippools,
		ippoolCache:  ippools.Cache(),
		podClient:    pods,
		podCache:     pods.Cache(),
	}

	ippools.OnChange(ctx, controllerName, handler.OnChange)
	ippools.OnRemove(ctx, controllerName, handler.OnRemove)

	return nil
}

func (h *Handler) OnChange(key string, ipPool *networkv1.IPPool) (*networkv1.IPPool, error) {
	if ipPool == nil || ipPool.DeletionTimestamp != nil {
		return ipPool, nil
	}

	klog.Infof("ippool configuration %s/%s has been changed: %+v", ipPool.Namespace, ipPool.Name, ipPool.Spec.IPv4Config)

	sets := labels.Set{
		ipPoolLabelKey: ipPool.Name,
	}
	pods, err := h.podCache.List(ipPool.Namespace, sets.AsSelector())
	if err != nil {
		return ipPool, err
	}

	// Each IPPool object should have an agent serves as its backend for the data plane
	if len(pods) == 0 {
		klog.Infof("ippool %s/%s has no agent found", ipPool.Namespace, ipPool.Name)

		agent, err := h.prepareAgentPod(ipPool)
		if err != nil {
			return ipPool, err
		}
		pod, err := h.podClient.Create(agent)
		if err != nil {
			return ipPool, err
		}
		klog.Infof("agent pod %s backing %s ippool has been created", pod.Name, ipPool.Name)
		ipPoolCpy := ipPool.DeepCopy()
		networkv1.Registered.SetStatus(ipPoolCpy, string(corev1.ConditionTrue))
		networkv1.Registered.Reason(ipPoolCpy, "")
		networkv1.Registered.Message(ipPoolCpy, "")
		return h.ippoolClient.UpdateStatus(ipPoolCpy)
	} else if len(pods) == 1 && networkv1.Registered.IsTrue(ipPool) && networkv1.Ready.GetStatus(ipPool) == "" {
		klog.Infof("ippool %s/%s has exactly one agent running", ipPool.Namespace, ipPool.Name)
		ipPoolCpy := ipPool.DeepCopy()
		networkv1.Ready.SetStatus(ipPoolCpy, string(corev1.ConditionTrue))
		networkv1.Ready.Reason(ipPoolCpy, "")
		networkv1.Ready.Message(ipPoolCpy, "")
		return h.ippoolClient.UpdateStatus(ipPoolCpy)
	} else {
		ipPoolCpy := ipPool.DeepCopy()
		networkv1.Ready.SetStatus(ipPoolCpy, string(corev1.ConditionFalse))
		networkv1.Ready.Reason(ipPoolCpy, "AgentMisconfigured")
		networkv1.Ready.Message(ipPoolCpy, fmt.Sprintf("ippool %s/%s has multiple agents", ipPool.Namespace, ipPool.Name))
		return h.ippoolClient.UpdateStatus(ipPoolCpy)
	}
}

func (h *Handler) OnRemove(key string, ipPool *networkv1.IPPool) (*networkv1.IPPool, error) {
	if ipPool == nil {
		return nil, nil
	}

	klog.Infof("ippool configuration %s/%s has been removed", ipPool.Namespace, ipPool.Name)

	return ipPool, nil
}

func (h *Handler) prepareAgentPod(ipPool *networkv1.IPPool) (*corev1.Pod, error) {
	var networks []Network
	network := Network{
		Namespace:     ipPool.Namespace,
		Name:          ipPool.Spec.NetworkName,
		InterfaceName: "eth1",
	}
	networks = append(networks, network)
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
				ipPoolLabelKey: ipPool.Name,
			},
			GenerateName: "vm-dhcp-controller-agent-",
			Namespace:    ipPool.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				ipPoolReference(ipPool),
			},
		},
		Spec: corev1.PodSpec{
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
						fmt.Sprintf("%s-%s-vdca", ipPool.Namespace, ipPool.Name),
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
				},
			},
		},
	}, nil
}

func ipPoolReference(ipPool *networkv1.IPPool) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: ipPool.APIVersion,
		Kind:       ipPool.Kind,
		Name:       ipPool.Name,
		UID:        ipPool.UID,
	}
}
