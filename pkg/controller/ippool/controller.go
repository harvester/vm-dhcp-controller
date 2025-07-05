package ippool

import (
	"context"
	"fmt"
	"reflect"

	"github.com/rancher/wrangler/pkg/kv"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	"k8s.io/apimachinery/pkg/api/errors" // k8serrors alias is used
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"encoding/json" // For serializing AgentNetConfig
	"os"            // For os.Getenv
	"sort"          // For sorting IPPools
	"strings"       // For argument parsing
)

const (
	controllerName = "vm-dhcp-ippool-controller"

	// AgentDeploymentNameSuffix is the suffix appended to the controller's fullname to get the agent deployment name.
	AgentDeploymentNameSuffix = "-agent"

	multusNetworksAnnotationKey         = "k8s.v1.cni.cncf.io/networks"
	holdIPPoolAgentUpgradeAnnotationKey = "network.harvesterhci.io/hold-ippool-agent-upgrade"

	vmDHCPControllerLabelKey = network.GroupName + "/vm-dhcp-controller"
	clusterNetworkLabelKey   = network.GroupName + "/clusternetwork"

	// Environment variable keys for agent configuration
	agentNetworkConfigsEnvKey = "AGENT_NETWORK_CONFIGS"
	agentIPPoolRefsEnvKey     = "IPPOOL_REFS_JSON"
)

var (
	runAsUserID  int64 = 0
	runAsGroupID int64 = 0
)

// AgentNetConfig defines the network configuration for a single interface in the agent pod.
type AgentNetConfig struct {
	InterfaceName string `json:"interfaceName"`
	ServerIP      string `json:"serverIP"`
	CIDR          string `json:"cidr"`
	IPPoolName    string `json:"ipPoolName"` // Namespaced name "namespace/name"
	IPPoolRef     string `json:"ipPoolRef"`  // Namespaced name "namespace/name" for direct reference
	NadName       string `json:"nadName"`    // Namespaced name "namespace/name" of the NAD
}

type Handler struct {
	cacheAllocator *cache.CacheAllocator
	ipAllocator      *ipam.IPAllocator
	metricsAllocator *metrics.MetricsAllocator

	ippoolController ctlnetworkv1.IPPoolController
	ippoolClient     ctlnetworkv1.IPPoolClient
	ippoolCache      ctlnetworkv1.IPPoolCache
	podClient        ctlcorev1.PodClient
	podCache         ctlcorev1.PodCache
	nadClient        ctlcniv1.NetworkAttachmentDefinitionClient
	nadCache         ctlcniv1.NetworkAttachmentDefinitionCache
	kubeClient       kubernetes.Interface
	agentNamespace   string // Namespace where the agent deployment resides
}

func Register(ctx context.Context, management *config.Management) error {
	ippools := management.HarvesterNetworkFactory.Network().V1alpha1().IPPool()
	pods := management.CoreFactory.Core().V1().Pod()
	nads := management.CniFactory.K8s().V1().NetworkAttachmentDefinition()

	handler := &Handler{
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
		kubeClient:       management.KubeClient,
		agentNamespace:   management.Namespace,
	}

	ctlnetworkv1.RegisterIPPoolStatusHandler(
		ctx,
		ippools,
		networkv1.CacheReady,
		"ippool-cache-builder",
		handler.BuildCache,
	)

	// Wrap OnChange and OnRemove to trigger global agent deployment reconciliation
	wrappedOnChange := func(key string, ipPool *networkv1.IPPool) (*networkv1.IPPool, error) {
		pool, err := handler.OnChangeInternal(key, ipPool) // Call the original logic
		if nerr := handler.reconcileAgentDeployment(context.TODO()); nerr != nil {
			logrus.Errorf("Error reconciling agent deployment after IPPool %s OnChange: %v", key, nerr)
			if err == nil { // If original OnChange was fine, return this new error
				err = nerr
			}
			// Potentially combine errors or prioritize one, for now, log and pass original/new error
		}
		return pool, err
	}

	wrappedOnRemove := func(key string, ipPool *networkv1.IPPool) (*networkv1.IPPool, error) {
		pool, err := handler.OnRemoveInternal(key, ipPool) // Call the original logic
		if nerr := handler.reconcileAgentDeployment(context.TODO()); nerr != nil {
			logrus.Errorf("Error reconciling agent deployment after IPPool %s OnRemove: %v", key, nerr)
			if err == nil { // If original OnRemove was fine, return this new error
				err = nerr
			}
		}
		return pool, err
	}

	ippools.OnChange(ctx, controllerName, wrappedOnChange)
	ippools.OnRemove(ctx, controllerName, wrappedOnRemove)

	// The initial reconciliation will be triggered by OnChange events when existing IPPools are synced.
	// Removing the explicit goroutine for initial reconciliation to prevent race conditions with cache sync.

	return nil
}

// OnChangeInternal contains the original logic of OnChange
func (h *Handler) OnChangeInternal(key string, ipPool *networkv1.IPPool) (*networkv1.IPPool, error) {
	if ipPool == nil || ipPool.DeletionTimestamp != nil {
		return nil, nil
	}

	logrus.Debugf("(ippool.OnChangeInternal) ippool configuration %s has been changed: %+v", key, ipPool.Spec.IPv4Config)

	if err := h.ensureNADLabels(ipPool); err != nil {
		return ipPool, err
	}

	ipPoolCpy := ipPool.DeepCopy()

	if ipPool.Spec.Paused != nil && *ipPool.Spec.Paused {
		logrus.Infof("(ippool.OnChangeInternal) ippool %s is paused, cleaning up local resources", key)
		if err := h.cleanup(ipPool); err != nil { // cleanup local IPAM etc.
			logrus.Errorf("Error during cleanup for paused IPPool %s: %v", key, err)
			// Continue to update status
		}
		networkv1.Stopped.True(ipPoolCpy)
		if !reflect.DeepEqual(ipPoolCpy, ipPool) {
			return h.ippoolClient.UpdateStatus(ipPoolCpy)
		}
		return ipPoolCpy, nil
	}
	networkv1.Stopped.False(ipPoolCpy)

	if !h.ipAllocator.IsNetworkInitialized(ipPool.Spec.NetworkName) {
		networkv1.CacheReady.False(ipPoolCpy)
		networkv1.CacheReady.Reason(ipPoolCpy, "NotInitialized")
		networkv1.CacheReady.Message(ipPoolCpy, "")
		if !reflect.DeepEqual(ipPoolCpy, ipPool) {
			logrus.Warningf("(ippool.OnChangeInternal) ipam for ippool %s/%s is not initialized", ipPool.Namespace, ipPool.Name)
			return h.ippoolClient.UpdateStatus(ipPoolCpy)
		}
	}

	ipv4Status := ipPoolCpy.Status.IPv4
	if ipv4Status == nil {
		ipv4Status = new(networkv1.IPv4Status)
	}

	used, err := h.ipAllocator.GetUsed(ipPool.Spec.NetworkName)
	if err != nil {
		return ipPool, fmt.Errorf("failed to get used IP count for %s: %w", ipPool.Spec.NetworkName, err)
	}
	ipv4Status.Used = used

	available, err := h.ipAllocator.GetAvailable(ipPool.Spec.NetworkName)
	if err != nil {
		return ipPool, fmt.Errorf("failed to get available IP count for %s: %w", ipPool.Spec.NetworkName, err)
	}
	ipv4Status.Available = available

	h.metricsAllocator.UpdateIPPoolUsed(key, ipPool.Spec.IPv4Config.CIDR, ipPool.Spec.NetworkName, used)
	h.metricsAllocator.UpdateIPPoolAvailable(key, ipPool.Spec.IPv4Config.CIDR, ipPool.Spec.NetworkName, available)

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
	if len(allocated) == 0 {
		allocated = nil
	}
	ipv4Status.Allocated = allocated
	ipPoolCpy.Status.IPv4 = ipv4Status

	if !reflect.DeepEqual(ipPoolCpy.Status, ipPool.Status) {
		logrus.Infof("(ippool.OnChangeInternal) updating ippool status %s/%s", ipPool.Namespace, ipPool.Name)
		ipPoolCpy.Status.LastUpdate = metav1.Now()
		return h.ippoolClient.UpdateStatus(ipPoolCpy)
	}

	return ipPoolCpy, nil
}

func (h *Handler) getAgentDeploymentName() string {
	agentDeploymentName := os.Getenv("AGENT_DEPLOYMENT_NAME")
	if agentDeploymentName == "" {
		logrus.Warn("AGENT_DEPLOYMENT_NAME env var not set, agent deployment updates may fail. Defaulting to a common pattern.")
		agentDeploymentName = "vm-dhcp-controller-agent" // Adjust if chart naming is different
	}
	return agentDeploymentName
}

func (h *Handler) getAgentContainerName() string {
	agentContainerName := os.Getenv("AGENT_CONTAINER_NAME")
	if agentContainerName == "" {
		logrus.Warnf("AGENT_CONTAINER_NAME env var not set. Defaulting to a common pattern.")
		agentContainerName = "vm-dhcp-controller-agent" // Adjust if chart naming is different
	}
	return agentContainerName
}

func (h *Handler) reconcileAgentDeployment(ctx context.Context) error {
	logrus.Info("Reconciling agent deployment for all active IPPools")

	agentDepName := h.getAgentDeploymentName()
	agentDepNamespace := h.agentNamespace
	agentContainerName := h.getAgentContainerName()

	allIPPools, err := h.ippoolCache.List(metav1.NamespaceAll, nil)
	if err != nil {
		return fmt.Errorf("failed to list IPPools: %w", err)
	}

	var activeIPPools []*networkv1.IPPool
	for _, ipPool := range allIPPools {
		if ipPool.DeletionTimestamp == nil && (ipPool.Spec.Paused == nil || !*ipPool.Spec.Paused) {
			// Check if IPv4Config itself is present and then check its fields.
			// The direct comparison ipPool.Spec.IPv4Config == nil was incorrect for a struct type.
			// The intention is to ensure essential fields within IPv4Config are populated.
			if ipPool.Spec.NetworkName == "" || ipPool.Spec.IPv4Config.ServerIP == "" || ipPool.Spec.IPv4Config.CIDR == "" {
				logrus.Warnf("IPPool %s/%s is active but missing required fields (NetworkName, IPv4Config.ServerIP, IPv4Config.CIDR), skipping for agent config", ipPool.Namespace, ipPool.Name)
				continue
			}
			activeIPPools = append(activeIPPools, ipPool)
		}
	}

	sort.SliceStable(activeIPPools, func(i, j int) bool {
		if activeIPPools[i].Namespace != activeIPPools[j].Namespace {
			return activeIPPools[i].Namespace < activeIPPools[j].Namespace
		}
		return activeIPPools[i].Name < activeIPPools[j].Name
	})

	var agentNetConfigs []AgentNetConfig
	var multusAnnotationItems []string
	var ipPoolRefs []string

	for i, ipPool := range activeIPPools {
		interfaceName := fmt.Sprintf("net%d", i)
		nadNamespace, nadName := kv.RSplit(ipPool.Spec.NetworkName, "/")
		if nadName == "" {
			nadName = nadNamespace
			nadNamespace = ipPool.Namespace
		}

		fullNadName := fmt.Sprintf("%s/%s", nadNamespace, nadName)
		multusAnnotationItems = append(multusAnnotationItems, fmt.Sprintf("%s@%s", fullNadName, interfaceName))

		poolRefKey := fmt.Sprintf("%s/%s", ipPool.Namespace, ipPool.Name)
		agentNetConfigs = append(agentNetConfigs, AgentNetConfig{
			InterfaceName: interfaceName,
			ServerIP:      ipPool.Spec.IPv4Config.ServerIP,
			CIDR:          ipPool.Spec.IPv4Config.CIDR,
			IPPoolName:    poolRefKey, // Original field, might be redundant with IPPoolRef
			IPPoolRef:     poolRefKey,
			NadName:       fullNadName,
		})
		ipPoolRefs = append(ipPoolRefs, poolRefKey)
	}

	agentNetConfigsJSONString := "[]"
	if len(agentNetConfigs) > 0 {
		jsonData, err := json.Marshal(agentNetConfigs)
		if err != nil {
			return fmt.Errorf("failed to marshal agent network configs: %w", err)
		}
		agentNetConfigsJSONString = string(jsonData)
	}

	ipPoolRefsJSONString := "[]"
	if len(ipPoolRefs) > 0 {
		jsonData, err := json.Marshal(ipPoolRefs)
		if err != nil {
			return fmt.Errorf("failed to marshal IPPool references: %w", err)
		}
		ipPoolRefsJSONString = string(jsonData)
	}

	deployment, err := h.kubeClient.AppsV1().Deployments(agentDepNamespace).Get(ctx, agentDepName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) { // Corrected: Use errors.IsNotFound
			logrus.Warnf("Agent deployment %s/%s not found. Cannot apply IPPool configurations.", agentDepNamespace, agentDepName)
			return nil // Nothing to update if deployment doesn't exist
		}
		return fmt.Errorf("failed to get agent deployment %s/%s: %w", agentDepNamespace, agentDepName, err)
	}

	needsUpdate := false
	depCopy := deployment.DeepCopy()

	// 1. Update Multus annotation
	desiredMultusAnnotation := strings.Join(multusAnnotationItems, ",")
	if depCopy.Spec.Template.Annotations == nil {
		depCopy.Spec.Template.Annotations = make(map[string]string)
	}
	currentMultusAnnotation := depCopy.Spec.Template.Annotations[multusNetworksAnnotationKey]

	if desiredMultusAnnotation == "" { // No active IPPools with valid config
		if _, exists := depCopy.Spec.Template.Annotations[multusNetworksAnnotationKey]; exists {
			delete(depCopy.Spec.Template.Annotations, multusNetworksAnnotationKey)
			needsUpdate = true
			logrus.Infof("Agent deployment %s/%s: Removing Multus annotation as no active/valid IPPools.", agentDepNamespace, agentDepName)
		}
	} else {
		if currentMultusAnnotation != desiredMultusAnnotation {
			depCopy.Spec.Template.Annotations[multusNetworksAnnotationKey] = desiredMultusAnnotation
			needsUpdate = true
			logrus.Infof("Agent deployment %s/%s: Updating Multus annotation to: %s", agentDepNamespace, agentDepName, desiredMultusAnnotation)
		}
	}
	if len(depCopy.Spec.Template.Annotations) == 0 { // Clean up if empty
	    depCopy.Spec.Template.Annotations = nil
	}


	// 2. Update container args and env vars
	containerUpdated := false
	for i, c := range depCopy.Spec.Template.Spec.Containers {
		if c.Name == agentContainerName {
			var newArgs []string
			for _, arg := range c.Args { // Remove only specific old args
				if !strings.HasPrefix(arg, "--nic") &&
					!strings.HasPrefix(arg, "--server-ip") &&
					!strings.HasPrefix(arg, "--cidr") &&
					!strings.HasPrefix(arg, "--ippool-ref") {
					newArgs = append(newArgs, arg)
				} else {
					needsUpdate = true // Indicate that an old arg was found and removed
				}
			}
			if len(newArgs) != len(c.Args) { // Check if args actually changed
			    depCopy.Spec.Template.Spec.Containers[i].Args = newArgs
			    // needsUpdate is already true if an old arg was removed
			}


			currentEnv := depCopy.Spec.Template.Spec.Containers[i].Env
			updatedEnv := h.updateEnvVar(currentEnv, agentNetworkConfigsEnvKey, agentNetConfigsJSONString, &needsUpdate)
			updatedEnv = h.updateEnvVar(updatedEnv, agentIPPoolRefsEnvKey, ipPoolRefsJSONString, &needsUpdate)
			updatedEnv = h.removeEnvVar(updatedEnv, "IPPOOL_REF", &needsUpdate) // Remove old single IPPOOL_REF

			if !reflect.DeepEqual(currentEnv, updatedEnv) {
				depCopy.Spec.Template.Spec.Containers[i].Env = updatedEnv
				// needsUpdate would have been set by helpers if changes occurred
			}
			containerUpdated = true
			break
		}
	}

	if !containerUpdated && (len(activeIPPools) > 0 || currentMultusAnnotation != "") {
		// Only warn if we expected to find the container but didn't,
		// and there are active pools or existing annotations to manage.
		logrus.Warnf("Agent container '%s' not found in deployment %s/%s. Cannot update args or env vars.", agentContainerName, agentDepNamespace, agentDepName)
	}

	if needsUpdate {
		logrus.Infof("Updating agent deployment %s/%s due to IPPool changes.", agentDepNamespace, agentDepName)
		_, err = h.kubeClient.AppsV1().Deployments(agentDepNamespace).Update(ctx, depCopy, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update agent deployment %s/%s: %w", agentDepNamespace, agentDepName, err)
		}
		logrus.Infof("Successfully updated agent deployment %s/%s", agentDepNamespace, agentDepName)
	} else {
		logrus.Infof("Agent deployment %s/%s is already up-to-date regarding IPPool configurations.", agentDepNamespace, agentDepName)
	}

	return nil
}

func (h *Handler) updateEnvVar(envVars []corev1.EnvVar, key, value string, needsUpdate *bool) []corev1.EnvVar {
	found := false
	for i, envVar := range envVars {
		if envVar.Name == key {
			if envVar.Value != value {
				envVars[i].Value = value
				*needsUpdate = true
				logrus.Debugf("Updated env var %s.", key)
			}
			found = true
			break
		}
	}
	if !found {
		envVars = append(envVars, corev1.EnvVar{Name: key, Value: value})
		*needsUpdate = true
		logrus.Debugf("Added env var %s.", key)
	}
	return envVars
}

func (h *Handler) removeEnvVar(envVars []corev1.EnvVar, key string, needsUpdate *bool) []corev1.EnvVar {
	var result []corev1.EnvVar
	removed := false
	for _, envVar := range envVars {
		if envVar.Name == key {
			*needsUpdate = true
			removed = true
			logrus.Debugf("Removed env var %s.", key)
			continue
		}
		result = append(result, envVar)
	}
	if !removed && len(result) == 0 && len(envVars) > 0 { // Handles case where all vars are removed
	    return nil
	}
	return result
}

// OnRemoveInternal contains the original logic of OnRemove
func (h *Handler) OnRemoveInternal(key string, ipPool *networkv1.IPPool) (*networkv1.IPPool, error) {
	if ipPool == nil {
		return nil, nil
	}

	logrus.Debugf("(ippool.OnRemove) ippool configuration %s/%s has been removed", ipPool.Namespace, ipPool.Name)

	if err := h.cleanup(ipPool); err != nil {
		return ipPool, err
	}

	return ipPool, nil
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
func (h *Handler) cleanup(ipPool *networkv1.IPPool) error {
	// AgentPodRef related checks and deletion logic removed as the controller no longer manages agent pods.
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
		if errors.IsNotFound(err) { // Corrected: Use errors.IsNotFound for NAD check
			logrus.Errorf("NetworkAttachmentDefinition %s/%s not found for IPPool %s/%s", nadNamespace, nadName, ipPool.Namespace, ipPool.Name)
			return fmt.Errorf("NAD %s/%s not found: %w", nadNamespace, nadName, err)
		}
		return fmt.Errorf("failed to get NAD %s/%s: %w", nadNamespace, nadName, err)
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

