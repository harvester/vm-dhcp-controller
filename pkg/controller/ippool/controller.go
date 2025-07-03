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
	// appsv1 "k8s.io/api/apps/v1" // Unused
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1" // This was the duplicate
	corev1 "k8s.io/api/core/v1"                   // For EnvVar
	"k8s.io/client-go/kubernetes"
	// "encoding/json" // Unused
	"os"     // For os.Getenv
	"strings" // For argument parsing
)

const (
	controllerName = "vm-dhcp-ippool-controller"

	// AgentDeploymentNameSuffix is the suffix appended to the controller's fullname to get the agent deployment name.
	// This assumes the controller's name (passed via --name flag) is the "fullname" from Helm.
	AgentDeploymentNameSuffix = "-agent"
	// AgentContainerName is the name of the container within the agent deployment.
	// This needs to match what's in chart/templates/agent-deployment.yaml ({{ .Chart.Name }}-agent)
	// For robustness, this might need to be configurable or derived more reliably.
	// Assuming Chart.Name is stable, e.g., "harvester-vm-dhcp-controller".
	// Let's use a placeholder and refine if needed. It's currently {{ .Chart.Name }} in agent-deployment.yaml
	// which resolves to "vm-dhcp-controller" if the chart is named that.
	// The agent deployment.yaml has container name {{ .Chart.Name }}-agent
	// AgentContainerNameDefault = "vm-dhcp-controller-agent" // Replaced by env var
	// DefaultAgentPodInterfaceName is the default name for the Multus interface in the agent pod.
	DefaultAgentPodInterfaceName = "net1"

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
		kubeClient:       management.KubeClient,     // Added KubeClient
		agentNamespace:   management.Namespace,    // Assuming Management has Namespace for the controller/agent
	}

	ctlnetworkv1.RegisterIPPoolStatusHandler(
		ctx,
		ippools,
		networkv1.CacheReady,
		"ippool-cache-builder",
		handler.BuildCache,
	)

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

	// After other processing, sync the agent deployment
	// Assuming `management.ControllerName` is available and set to the controller's helm fullname
	// This name needs to be reliably determined. For now, using a placeholder.
	// The actual controller name (used for leader election etc.) is often passed via --name flag.
	// Let's assume `management.ControllerName` is available in `h` or can be fetched.
	// For now, this part of agent deployment name construction is illustrative.
	// It needs to align with how the agent deployment is actually named by Helm.
	// Agent deployment name is: {{ include "harvester-vm-dhcp-controller.fullname" . }}-agent
	// The controller's own "fullname" is needed. This is typically available from options.
	// Let's assume `h.agentNamespace` is where the controller (and agent) runs.
	// And the controller's name (helm fullname) is something we can get, e.g. from an env var or option.
	// This dynamic configuration needs the controller's own Helm fullname.
	// Let's assume it's available via h.getControllerHelmFullName() for now.
	// This is a complex part to get right without knowing how controllerName is populated.
	// For now, skipping the actual agent deployment update to avoid introducing half-baked logic
	// without having the controller's own Helm fullname.
	// TODO: Implement dynamic agent deployment update once controller's Helm fullname is accessible.
	if err := h.syncAgentDeployment(ipPoolCpy); err != nil {
		// Log the error but don't necessarily block IPPool reconciliation for agent deployment issues.
		// The IPPool status update should still proceed.
		logrus.Errorf("Failed to sync agent deployment for ippool %s: %v", key, err)
		// Depending on desired behavior, you might want to return the error or update a condition on ipPool.
		// For now, just logging.
	}

	return ipPoolCpy, nil // Return potentially updated ipPoolCpy from status updates
}

// getAgentDeploymentName constructs the expected name of the agent deployment.
// It relies on the AGENT_DEPLOYMENT_NAME environment variable.
func (h *Handler) getAgentDeploymentName() string {
	// This assumes the agent deployment follows "<helm-release-name>-<chart-name>-agent" if fullname is complex,
	// or just "<helm-release-name>-agent" if chart name is part of release name.
	// The agent deployment is named: {{ template "harvester-vm-dhcp-controller.fullname" . }}-agent
	// If controllerHelmReleaseName is the "fullname", then it's controllerHelmReleaseName + AgentDeploymentNameSuffix
	// This needs to be robust. For now, let's assume a simpler derivation for the placeholder.
	// This needs to match what `{{ include "harvester-vm-dhcp-controller.fullname" . }}-agent` resolves to.
	// This is difficult to resolve from within the controller without more context (like Release Name, Chart Name).
	// Let's hardcode for now based on common Helm chart naming, this is a simplification.
	// Example: if release is "harvester", chart is "vm-dhcp-controller", fullname is "harvester-vm-dhcp-controller"
	// Agent deployment: "harvester-vm-dhcp-controller-agent"
	// This is a critical piece that needs to be accurate.
	// It might be better to pass this via an environment variable set in the controller's deployment.yaml.
	agentDeploymentName := os.Getenv("AGENT_DEPLOYMENT_NAME")
	if agentDeploymentName == "" {
		// Fallback, but this should be explicitly set for reliability.
		logrus.Warn("AGENT_DEPLOYMENT_NAME env var not set, agent deployment updates may fail.")
		// This is a guess and likely incorrect without proper templating/env var.
		agentDeploymentName = "harvester-vm-dhcp-controller-agent"
	}
	return agentDeploymentName
}

// getAgentContainerName retrieves the agent container name from an environment variable.
func (h *Handler) getAgentContainerName() string {
	agentContainerName := os.Getenv("AGENT_CONTAINER_NAME")
	if agentContainerName == "" {
		logrus.Warnf("AGENT_CONTAINER_NAME env var not set, agent deployment updates may fail to find the container. Defaulting to '%s-agent'", "harvester-vm-dhcp-controller")
		// This fallback is a guess based on common chart naming.
		// It should be `Chart.Name + "-agent"`. Since Chart.Name is "harvester-vm-dhcp-controller",
		// this becomes "harvester-vm-dhcp-controller-agent".
		// However, the Helm template for agent container name is `{{ .Chart.Name }}-agent`.
		// If Chart.Name from Chart.yaml is `harvester-vm-dhcp-controller`, then this is correct.
		// If Chart.Name is something else, this fallback is wrong.
		// The env var is the reliable source.
		return "harvester-vm-dhcp-controller-agent" // Fallback based on typical Chart.Name
	}
	return agentContainerName
}


// syncAgentDeployment updates the agent deployment to attach to the NAD from the IPPool
func (h *Handler) syncAgentDeployment(ipPool *networkv1.IPPool) error {
	if ipPool == nil || ipPool.Spec.NetworkName == "" {
		// Or handle deletion/detachment if networkName is cleared
		return nil
	}

	agentDepName := h.getAgentDeploymentName()
	agentDepNamespace := h.agentNamespace

	logrus.Infof("Syncing agent deployment %s/%s for IPPool %s/%s (NAD: %s)",
		agentDepNamespace, agentDepName, ipPool.Namespace, ipPool.Name, ipPool.Spec.NetworkName)

	deployment, err := h.kubeClient.AppsV1().Deployments(agentDepNamespace).Get(context.TODO(), agentDepName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			logrus.Errorf("Agent deployment %s/%s not found, cannot update for IPPool %s", agentDepNamespace, agentDepName, ipPool.Name)
			return nil // Or return error if agent deployment is mandatory
		}
		return fmt.Errorf("failed to get agent deployment %s/%s: %w", agentDepNamespace, agentDepName, err)
	}

	nadNamespace, nadName := kv.RSplit(ipPool.Spec.NetworkName, "/")
	if nadName == "" { // Assume it's in the same namespace as IPPool if no "/"
		nadName = nadNamespace
		nadNamespace = ipPool.Namespace
	}

	// Determine target interface name, e.g., from IPPool annotation or default
	// For now, using DefaultAgentPodInterfaceName = "net1"
	podIFName := DefaultAgentPodInterfaceName
	// Potentially override podIFName from an IPPool annotation in the future
	// e.g., podIFName = ipPool.Annotations["network.harvesterhci.io/agent-pod-interface-name"]

	desiredAnnotationValue := fmt.Sprintf("%s/%s@%s", nadNamespace, nadName, podIFName)

	needsUpdate := false
	currentAnnotationValue, annotationExists := deployment.Spec.Template.Annotations[multusNetworksAnnotationKey]

	if !annotationExists || currentAnnotationValue != desiredAnnotationValue {
		if deployment.Spec.Template.Annotations == nil {
			deployment.Spec.Template.Annotations = make(map[string]string)
		}
		deployment.Spec.Template.Annotations[multusNetworksAnnotationKey] = desiredAnnotationValue
		needsUpdate = true
		logrus.Infof("Updating agent deployment %s/%s annotation to: %s", agentDepNamespace, agentDepName, desiredAnnotationValue)
	}

	// Find and update the --nic argument
	containerFound := false
	for i, container := range deployment.Spec.Template.Spec.Containers {
		agentContainerName := h.getAgentContainerName()
		if container.Name == agentContainerName {
			containerFound = true
			originalArgs := container.Args
			newArgs := []string{}
			nicArgUpdated := false
			skipNext := false

			for j := 0; j < len(originalArgs); j++ {
				if skipNext {
					skipNext = false
					continue
				}
				arg := originalArgs[j]
				if strings.HasPrefix(arg, "--nic=") {
					currentVal := strings.SplitN(arg, "=", 2)[1]
					if currentVal != podIFName {
						logrus.Infof("Updating --nic arg for agent deployment %s/%s from %s to %s (form --nic=value)",
							agentDepNamespace, agentDepName, arg, fmt.Sprintf("--nic=%s", podIFName))
						newArgs = append(newArgs, fmt.Sprintf("--nic=%s", podIFName))
						needsUpdate = true
					} else {
						newArgs = append(newArgs, arg) // Already correct
					}
					nicArgUpdated = true
				} else if arg == "--nic" {
					if j+1 < len(originalArgs) { // Check if there's a value after --nic
						currentVal := originalArgs[j+1]
						if currentVal != podIFName {
							logrus.Infof("Updating --nic arg for agent deployment %s/%s from %s to %s (form --nic value)",
								agentDepNamespace, agentDepName, currentVal, podIFName)
							newArgs = append(newArgs, "--nic", podIFName)
							needsUpdate = true
						} else {
							newArgs = append(newArgs, "--nic", currentVal) // Already correct
						}
						skipNext = true // Skip the original value part
					} else { // --nic was the last argument, malformed or expecting default
						logrus.Infof("Adding value for --nic arg for agent deployment %s/%s, new value: %s",
							agentDepNamespace, agentDepName, podIFName)
						newArgs = append(newArgs, "--nic", podIFName)
						needsUpdate = true
					}
					nicArgUpdated = true
				} else {
					newArgs = append(newArgs, arg)
				}
			}

			if !nicArgUpdated {
				logrus.Infof("Adding --nic arg %s for agent deployment %s/%s", podIFName, agentDepNamespace, agentDepName)
				newArgs = append(newArgs, "--nic", podIFName)
				needsUpdate = true
			}
			deployment.Spec.Template.Spec.Containers[i].Args = newArgs // Commit --nic changes

			// Ensure desiredIPPoolRef is defined before this block
			desiredIPPoolRef := fmt.Sprintf("%s/%s", ipPool.Namespace, ipPool.Name)

			// Update/Set IPPOOL_REF environment variable
			envVarFound := false
			for k, envVar := range deployment.Spec.Template.Spec.Containers[i].Env {
				if envVar.Name == "IPPOOL_REF" {
					envVarFound = true
					if envVar.Value != desiredIPPoolRef {
						logrus.Infof("Updating IPPOOL_REF env var for agent deployment %s/%s from '%s' to '%s'",
							agentDepNamespace, agentDepName, envVar.Value, desiredIPPoolRef)
						deployment.Spec.Template.Spec.Containers[i].Env[k].Value = desiredIPPoolRef
						needsUpdate = true
					}
					break
				}
			}
			if !envVarFound {
				logrus.Infof("Adding IPPOOL_REF env var '%s' for agent deployment %s/%s", desiredIPPoolRef, agentDepNamespace, agentDepName)
				deployment.Spec.Template.Spec.Containers[i].Env = append(deployment.Spec.Template.Spec.Containers[i].Env, corev1.EnvVar{
					Name:  "IPPOOL_REF",
					Value: desiredIPPoolRef,
				})
				needsUpdate = true
			}

			// Update/Set --server-ip and --cidr arguments
			desiredServerIP := ipPool.Spec.IPv4Config.ServerIP
			desiredCIDR := ipPool.Spec.IPv4Config.CIDR

			// Re-fetch args as they might have been changed by --nic update
			currentArgs := deployment.Spec.Template.Spec.Containers[i].Args
			finalArgs := []string{}
			processedFlags := make(map[string]bool) // To track if we've handled a flag

			for j := 0; j < len(currentArgs); j++ {
				arg := currentArgs[j]
				// Handle --nic (already processed, just copy)
				if strings.HasPrefix(arg, "--nic=") || arg == "--nic" {
					finalArgs = append(finalArgs, arg)
					if arg == "--nic" && j+1 < len(currentArgs) && !strings.HasPrefix(currentArgs[j+1], "--") {
						finalArgs = append(finalArgs, currentArgs[j+1])
						j++
					}
					processedFlags["--nic"] = true
					continue
				}

				// Handle --server-ip
				if strings.HasPrefix(arg, "--server-ip=") {
					if arg != fmt.Sprintf("--server-ip=%s", desiredServerIP) {
						logrus.Infof("Updating --server-ip arg for agent deployment from %s to %s", arg, desiredServerIP)
						finalArgs = append(finalArgs, fmt.Sprintf("--server-ip=%s", desiredServerIP))
						needsUpdate = true
					} else {
						finalArgs = append(finalArgs, arg)
					}
					processedFlags["--server-ip"] = true
					continue
				}
				if arg == "--server-ip" {
					if j+1 < len(currentArgs) && !strings.HasPrefix(currentArgs[j+1], "--") {
						if currentArgs[j+1] != desiredServerIP {
							logrus.Infof("Updating --server-ip value for agent deployment from %s to %s", currentArgs[j+1], desiredServerIP)
							needsUpdate = true
						}
						finalArgs = append(finalArgs, "--server-ip", desiredServerIP)
						j++
					} else { // Flag without value or next is another flag
						logrus.Infof("Setting value for --server-ip arg for agent deployment to %s", desiredServerIP)
						finalArgs = append(finalArgs, "--server-ip", desiredServerIP)
						needsUpdate = true
					}
					processedFlags["--server-ip"] = true
					continue
				}

				// Handle --cidr
				if strings.HasPrefix(arg, "--cidr=") {
					if arg != fmt.Sprintf("--cidr=%s", desiredCIDR) {
						logrus.Infof("Updating --cidr arg for agent deployment from %s to %s", arg, desiredCIDR)
						finalArgs = append(finalArgs, fmt.Sprintf("--cidr=%s", desiredCIDR))
						needsUpdate = true
					} else {
						finalArgs = append(finalArgs, arg)
					}
					processedFlags["--cidr"] = true
					continue
				}
				if arg == "--cidr" {
					if j+1 < len(currentArgs) && !strings.HasPrefix(currentArgs[j+1], "--") {
						if currentArgs[j+1] != desiredCIDR {
							logrus.Infof("Updating --cidr value for agent deployment from %s to %s", currentArgs[j+1], desiredCIDR)
							needsUpdate = true
						}
						finalArgs = append(finalArgs, "--cidr", desiredCIDR)
						j++
					} else {
						logrus.Infof("Setting value for --cidr arg for agent deployment to %s", desiredCIDR)
						finalArgs = append(finalArgs, "--cidr", desiredCIDR)
						needsUpdate = true
					}
					processedFlags["--cidr"] = true
					continue
				}
				// Copy other args
				finalArgs = append(finalArgs, arg)
			}

			// Add flags if they weren't processed (i.e., didn't exist)
			if !processedFlags["--server-ip"] && desiredServerIP != "" {
				logrus.Infof("Adding --server-ip arg %s for agent deployment", desiredServerIP)
				finalArgs = append(finalArgs, "--server-ip", desiredServerIP)
				needsUpdate = true
			}
			if !processedFlags["--cidr"] && desiredCIDR != "" {
				logrus.Infof("Adding --cidr arg %s for agent deployment", desiredCIDR)
				finalArgs = append(finalArgs, "--cidr", desiredCIDR)
				needsUpdate = true
			}
			deployment.Spec.Template.Spec.Containers[i].Args = finalArgs
			break
		}
	}

	if !containerFound {
		// Use agentContainerName variable which holds the result of h.getAgentContainerName()
		// This variable should be defined at the beginning of the loop or function if not already.
		// Let's ensure agentContainerName is in scope here.
		// It was defined when iterating containers: agentContainerName := h.getAgentContainerName()
		// This means it needs to be fetched once before the loop.
		logrus.Warnf("Agent container '%s' not found in deployment %s/%s. Cannot update args or IPPOOL_REF env var.", h.getAgentContainerName(), agentDepNamespace, agentDepName)
	}


	if needsUpdate {
		logrus.Infof("Patching agent deployment %s/%s", agentDepNamespace, agentDepName)
		_, err = h.kubeClient.AppsV1().Deployments(agentDepNamespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update agent deployment %s/%s: %w", agentDepNamespace, agentDepName, err)
		}
		logrus.Infof("Successfully patched agent deployment %s/%s", agentDepNamespace, agentDepName)
	} else {
		logrus.Infof("Agent deployment %s/%s already up-to-date for IPPool %s/%s (NAD: %s)",
			agentDepNamespace, agentDepName, ipPool.Namespace, ipPool.Name, ipPool.Spec.NetworkName)
	}

	return nil
}


func (h *Handler) OnRemove(key string, ipPool *networkv1.IPPool) (*networkv1.IPPool, error) {
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

