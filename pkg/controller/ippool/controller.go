package ippool

import (
	"context"
	"fmt"
	"reflect"

	"github.com/rancher/wrangler/v3/pkg/kv"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io"
	networkv1 "github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/harvester/vm-dhcp-controller/pkg/cache"
	"github.com/harvester/vm-dhcp-controller/pkg/config"
	ctlcorev1 "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/core/v1"
	ctlcniv1 "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/k8s.cni.cncf.io/v1"
	ctlnetworkv1 "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/network.harvesterhci.io/v1alpha1"
	"github.com/harvester/vm-dhcp-controller/pkg/ipam"
	"github.com/harvester/vm-dhcp-controller/pkg/metrics"
	"github.com/harvester/vm-dhcp-controller/pkg/util"
)

const (
	controllerName = "vm-dhcp-ippool-controller"

	multusNetworksAnnotationKey         = "k8s.v1.cni.cncf.io/networks"
	holdIPPoolAgentUpgradeAnnotationKey = "network.harvesterhci.io/hold-ippool-agent-upgrade"

	vmDHCPControllerLabelKey = network.GroupName + "/vm-dhcp-controller"
	clusterNetworkLabelKey   = network.GroupName + "/clusternetwork"

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

	cacheAllocator   *cache.CacheAllocator
	ipAllocator      *ipam.IPAllocator
	metricsAllocator *metrics.MetricsAllocator

	ippoolController ctlnetworkv1.IPPoolController
	ippoolClient     ctlnetworkv1.IPPoolClient
	ippoolCache      ctlnetworkv1.IPPoolCache
	podClient        ctlcorev1.PodClient
	podCache         ctlcorev1.PodCache
	nadClient        ctlcniv1.NetworkAttachmentDefinitionClient
	nadCache         ctlcniv1.NetworkAttachmentDefinitionCache
}

func Register(ctx context.Context, management *config.Management) error {
	ippools := management.HarvesterNetworkFactory.Network().V1alpha1().IPPool()
	pods := management.CoreFactory.Core().V1().Pod()
	nads := management.CniFactory.K8s().V1().NetworkAttachmentDefinition()

	handler := &Handler{
		agentNamespace:          management.Options.AgentNamespace,
		agentImage:              management.Options.AgentImage,
		agentServiceAccountName: management.Options.AgentServiceAccountName,
		noAgent:                 management.Options.NoAgent,
		noDHCP:                  management.Options.NoDHCP,

		cacheAllocator:   management.CacheAllocator,
		ipAllocator:      management.IPAllocator,
		metricsAllocator: management.MetricsAllocator,

		ippoolController: ippools,
		ippoolClient:     ippools,
		ippoolCache:      ippools.Cache(),
		podClient:        pods,
		podCache:         pods.Cache(),
		nadClient:        nads,
		nadCache:         nads.Cache(),
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
		networkv1.CacheReady,
		"ippool-cache-builder",
		handler.BuildCache,
	)
	ctlnetworkv1.RegisterIPPoolStatusHandler(
		ctx,
		ippools,
		networkv1.AgentReady,
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
				Namespace: pod.Labels[util.IPPoolNamespaceLabelKey],
				Name:      pod.Labels[util.IPPoolNameLabelKey],
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

	logrus.Debugf("(ippool.OnChange) ippool configuration %s has been changed: %+v", key, ipPool.Spec.IPv4Config)

	// Build the relationship between IPPool and NetworkAttachmentDefinition for VirtualMachineNetworkConfig to reference
	if err := h.ensureNADLabels(ipPool); err != nil {
		return ipPool, err
	}

	ipPoolCpy := ipPool.DeepCopy()

	// Check if the IPPool is administratively disabled
	if ipPool.Spec.Paused != nil && *ipPool.Spec.Paused {
		logrus.Infof("(ippool.OnChange) try to cleanup cache and agent for ippool %s", key)
		if err := h.cleanup(ipPool); err != nil {
			return ipPool, err
		}
		ipPoolCpy.Status.AgentPodRef = nil
		networkv1.Stopped.True(ipPoolCpy)
		if !reflect.DeepEqual(ipPoolCpy, ipPool) {
			return h.ippoolClient.UpdateStatus(ipPoolCpy)
		}
		return ipPool, nil
	}
	networkv1.Stopped.False(ipPoolCpy)

	if !h.ipAllocator.IsNetworkInitialized(ipPool.Spec.NetworkName) {
		networkv1.CacheReady.False(ipPoolCpy)
		networkv1.CacheReady.Reason(ipPoolCpy, "NotInitialized")
		networkv1.CacheReady.Message(ipPoolCpy, "")
		if !reflect.DeepEqual(ipPoolCpy, ipPool) {
			logrus.Warningf("(ippool.OnChange) ipam for ippool %s/%s is not initialized", ipPool.Namespace, ipPool.Name)
			return h.ippoolClient.UpdateStatus(ipPoolCpy)
		}
	}

	// Update IPPool status based on up-to-date IPAM

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

	// Update IPPool metrics
	h.metricsAllocator.UpdateIPPoolUsed(
		key,
		ipPool.Spec.IPv4Config.CIDR,
		ipPool.Spec.NetworkName,
		used,
	)
	h.metricsAllocator.UpdateIPPoolAvailable(key,
		ipPool.Spec.IPv4Config.CIDR,
		ipPool.Spec.NetworkName,
		available,
	)

	allocated := ipv4Status.Allocated
	if allocated == nil {
		allocated = make(map[string]string)
	}
	if util.IsIPInBetweenOf(ipPool.Spec.IPv4Config.ServerIP, ipPool.Spec.IPv4Config.Pool.Start, ipPool.Spec.IPv4Config.Pool.End) {
		allocated[ipPool.Spec.IPv4Config.ServerIP] = util.ReservedMark
	}
	if util.IsIPInBetweenOf(ipPool.Spec.IPv4Config.Router, ipPool.Spec.IPv4Config.Pool.Start, ipPool.Spec.IPv4Config.Pool.End) {
		allocated[ipPool.Spec.IPv4Config.Router] = util.ReservedMark
	}
	for _, eIP := range ipPool.Spec.IPv4Config.Pool.Exclude {
		allocated[eIP] = util.ExcludedMark
	}
	// For DeepEqual
	if len(allocated) == 0 {
		allocated = nil
	}
	ipv4Status.Allocated = allocated

	ipPoolCpy.Status.IPv4 = ipv4Status

	if !reflect.DeepEqual(ipPoolCpy, ipPool) {
		logrus.Infof("(ippool.OnChange) update ippool %s/%s", ipPool.Namespace, ipPool.Name)
		ipPoolCpy.Status.LastUpdate = metav1.Now()
		return h.ippoolClient.UpdateStatus(ipPoolCpy)
	}

	return ipPool, nil
}

func (h *Handler) OnRemove(key string, ipPool *networkv1.IPPool) (*networkv1.IPPool, error) {
	if ipPool == nil {
		return nil, nil
	}

	logrus.Debugf("(ippool.OnRemove) ippool configuration %s/%s has been removed", ipPool.Namespace, ipPool.Name)

	if h.noAgent {
		return ipPool, nil
	}

	if err := h.cleanup(ipPool); err != nil {
		return ipPool, err
	}

	return ipPool, nil
}

// DeployAgent reconciles ipPool and ensures there's an agent pod for it. The
// returned status reports whether an agent pod is registered.
func (h *Handler) DeployAgent(ipPool *networkv1.IPPool, status networkv1.IPPoolStatus) (networkv1.IPPoolStatus, error) {
	logrus.Debugf("(ippool.DeployAgent) deploy agent for ippool %s/%s", ipPool.Namespace, ipPool.Name)

	if ipPool.Spec.Paused != nil && *ipPool.Spec.Paused {
		return status, fmt.Errorf("ippool %s/%s was administratively disabled", ipPool.Namespace, ipPool.Name)
	}

	if h.noAgent {
		return status, nil
	}

	nadNamespace, nadName := kv.RSplit(ipPool.Spec.NetworkName, "/")
	nad, err := h.nadCache.Get(nadNamespace, nadName)
	if err != nil {
		return status, err
	}

	if nad.Labels == nil {
		return status, fmt.Errorf("could not find clusternetwork for nad %s", ipPool.Spec.NetworkName)
	}

	clusterNetwork, ok := nad.Labels[clusterNetworkLabelKey]
	if !ok {
		return status, fmt.Errorf("could not find clusternetwork for nad %s", ipPool.Spec.NetworkName)
	}

	if ipPool.Status.AgentPodRef != nil {
		status.AgentPodRef.Image = h.getAgentImage(ipPool)
		pod, err := h.podCache.Get(ipPool.Status.AgentPodRef.Namespace, ipPool.Status.AgentPodRef.Name)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return status, err
			}

			logrus.Warningf("(ippool.DeployAgent) agent pod %s missing, redeploying", ipPool.Status.AgentPodRef.Name)
		} else {
			if pod.DeletionTimestamp != nil {
				return status, fmt.Errorf("agent pod %s marked for deletion", ipPool.Status.AgentPodRef.Name)
			}

			if pod.GetUID() != ipPool.Status.AgentPodRef.UID {
				return status, fmt.Errorf("agent pod %s uid mismatch", ipPool.Status.AgentPodRef.Name)
			}

			return status, nil
		}
	}

	agent, err := prepareAgentPod(ipPool, h.noDHCP, h.agentNamespace, clusterNetwork, h.agentServiceAccountName, h.agentImage)
	if err != nil {
		return status, err
	}

	if status.AgentPodRef == nil {
		status.AgentPodRef = new(networkv1.PodReference)
	}

	status.AgentPodRef.Image = h.agentImage.String()

	agentPod, err := h.podClient.Create(agent)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return status, nil
		}
		return status, err
	}

	logrus.Infof("(ippool.DeployAgent) agent for ippool %s/%s has been deployed", ipPool.Namespace, ipPool.Name)

	status.AgentPodRef.Namespace = agentPod.Namespace
	status.AgentPodRef.Name = agentPod.Name
	status.AgentPodRef.UID = agentPod.GetUID()

	return status, nil
}

// BuildCache reconciles ipPool and initializes the IPAM and MAC caches for it.
// The source information comes from both ipPool's spec and status. Since
// IPPool objects are deemed source of truths, BuildCache honors the state and
// use it to load up internal caches. The returned status reports whether both
// caches are fully initialized.
func (h *Handler) BuildCache(ipPool *networkv1.IPPool, status networkv1.IPPoolStatus) (networkv1.IPPoolStatus, error) {
	logrus.Debugf("(ippool.BuildCache) build ipam for ippool %s/%s", ipPool.Namespace, ipPool.Name)

	if ipPool.Spec.Paused != nil && *ipPool.Spec.Paused {
		return status, fmt.Errorf("ippool %s/%s was administratively disabled", ipPool.Namespace, ipPool.Name)
	}

	if networkv1.CacheReady.IsTrue(ipPool) {
		return status, nil
	}

	logrus.Infof("(ippool.BuildCache) initialize ipam for ippool %s/%s", ipPool.Namespace, ipPool.Name)
	if err := h.ipAllocator.NewIPSubnet(
		ipPool.Spec.NetworkName,
		ipPool.Spec.IPv4Config.CIDR,
		ipPool.Spec.IPv4Config.Pool.Start,
		ipPool.Spec.IPv4Config.Pool.End,
	); err != nil {
		return status, err
	}

	logrus.Infof("(ippool.BuildCache) initialize mac cache for ippool %s/%s", ipPool.Namespace, ipPool.Name)
	if err := h.cacheAllocator.NewMACSet(ipPool.Spec.NetworkName); err != nil {
		return status, err
	}

	// Revoke server IP address in IPAM
	if err := h.ipAllocator.RevokeIP(ipPool.Spec.NetworkName, ipPool.Spec.IPv4Config.ServerIP); err != nil {
		return status, err
	}
	logrus.Debugf("(ippool.BuildCache) server ip %s was revoked in ipam %s", ipPool.Spec.IPv4Config.ServerIP, ipPool.Spec.NetworkName)

	// Revoke router IP address in IPAM
	if err := h.ipAllocator.RevokeIP(ipPool.Spec.NetworkName, ipPool.Spec.IPv4Config.Router); err != nil {
		return status, err
	}
	logrus.Debugf("(ippool.BuildCache) router ip %s was revoked in ipam %s", ipPool.Spec.IPv4Config.Router, ipPool.Spec.NetworkName)

	// Revoke excluded IP addresses in IPAM
	for _, eIP := range ipPool.Spec.IPv4Config.Pool.Exclude {
		if err := h.ipAllocator.RevokeIP(ipPool.Spec.NetworkName, eIP); err != nil {
			return status, err
		}
		logrus.Infof("(ippool.BuildCache) excluded ip %s was revoked in ipam %s", eIP, ipPool.Spec.NetworkName)
	}

	// (Re)build caches from IPPool status
	if ipPool.Status.IPv4 != nil {
		for ip, mac := range ipPool.Status.IPv4.Allocated {
			if mac == util.ExcludedMark || mac == util.ReservedMark {
				continue
			}
			if _, err := h.ipAllocator.AllocateIP(ipPool.Spec.NetworkName, ip); err != nil {
				return status, err
			}
			if err := h.cacheAllocator.AddMAC(ipPool.Spec.NetworkName, mac, ip); err != nil {
				return status, err
			}
			logrus.Infof("(ippool.BuildCache) previously allocated ip %s was re-allocated in ipam %s", ip, ipPool.Spec.NetworkName)
		}
	}

	logrus.Infof("(ippool.BuildCache) ipam and mac cache %s for ippool %s/%s has been updated", ipPool.Spec.NetworkName, ipPool.Namespace, ipPool.Name)

	return status, nil
}

// MonitorAgent reconciles ipPool and keeps an eye on the agent pod. If the
// running agent pod does not match to the one record in ipPool's status,
// MonitorAgent tries to delete it. The returned status reports whether the
// agent pod is ready.
func (h *Handler) MonitorAgent(ipPool *networkv1.IPPool, status networkv1.IPPoolStatus) (networkv1.IPPoolStatus, error) {
	logrus.Debugf("(ippool.MonitorAgent) monitor agent for ippool %s/%s", ipPool.Namespace, ipPool.Name)

	if ipPool.Spec.Paused != nil && *ipPool.Spec.Paused {
		return status, fmt.Errorf("ippool %s/%s was administratively disabled", ipPool.Namespace, ipPool.Name)
	}

	if h.noAgent {
		return status, nil
	}

	if ipPool.Status.AgentPodRef == nil {
		return status, fmt.Errorf("agent for ippool %s/%s is not deployed", ipPool.Namespace, ipPool.Name)
	}

	agentPod, err := h.podCache.Get(ipPool.Status.AgentPodRef.Namespace, ipPool.Status.AgentPodRef.Name)
	if err != nil {
		return status, err
	}

	if agentPod.GetUID() != ipPool.Status.AgentPodRef.UID || agentPod.Spec.Containers[0].Image != ipPool.Status.AgentPodRef.Image {
		if agentPod.DeletionTimestamp != nil {
			return status, fmt.Errorf("agent pod %s marked for deletion", agentPod.Name)
		}

		if err := h.podClient.Delete(agentPod.Namespace, agentPod.Name, &metav1.DeleteOptions{}); err != nil {
			return status, err
		}

		return status, fmt.Errorf("agent pod %s obsolete and purged", agentPod.Name)
	}

	if !isPodReady(agentPod) {
		return status, fmt.Errorf("agent pod %s not ready", agentPod.Name)
	}

	return status, nil
}

func isPodReady(pod *corev1.Pod) bool {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

func (h *Handler) getAgentImage(ipPool *networkv1.IPPool) string {
	_, ok := ipPool.Annotations[holdIPPoolAgentUpgradeAnnotationKey]
	if ok {
		return ipPool.Status.AgentPodRef.Image
	}
	return h.agentImage.String()
}

func (h *Handler) cleanup(ipPool *networkv1.IPPool) error {
	if ipPool.Status.AgentPodRef == nil {
		return nil
	}

	logrus.Infof("(ippool.cleanup) remove the backing agent %s/%s for ippool %s/%s", ipPool.Status.AgentPodRef.Namespace, ipPool.Status.AgentPodRef.Name, ipPool.Namespace, ipPool.Name)
	if err := h.podClient.Delete(ipPool.Status.AgentPodRef.Namespace, ipPool.Status.AgentPodRef.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	h.ipAllocator.DeleteIPSubnet(ipPool.Spec.NetworkName)
	h.cacheAllocator.DeleteMACSet(ipPool.Spec.NetworkName)
	h.metricsAllocator.DeleteIPPool(
		ipPool.Spec.NetworkName,
		ipPool.Spec.IPv4Config.CIDR,
		ipPool.Spec.NetworkName,
	)

	return nil
}

func (h *Handler) ensureNADLabels(ipPool *networkv1.IPPool) error {
	nadNamespace, nadName := kv.RSplit(ipPool.Spec.NetworkName, "/")
	nad, err := h.nadCache.Get(nadNamespace, nadName)
	if err != nil {
		return err
	}

	nadCpy := nad.DeepCopy()
	if nadCpy.Labels == nil {
		nadCpy.Labels = make(map[string]string)
	}
	nadCpy.Labels[util.IPPoolNamespaceLabelKey] = ipPool.Namespace
	nadCpy.Labels[util.IPPoolNameLabelKey] = ipPool.Name

	if !reflect.DeepEqual(nadCpy, nad) {
		logrus.Infof("(ippool.ensureNADLabels) update nad %s/%s", nad.Namespace, nad.Name)
		if _, err := h.nadClient.Update(nadCpy); err != nil {
			return err
		}
	}

	return nil
}
