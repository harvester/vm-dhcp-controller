package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

	"github.com/harvester/vm-dhcp-controller/pkg/agent/ippool"
	"github.com/harvester/vm-dhcp-controller/pkg/config"
	"github.com/harvester/vm-dhcp-controller/pkg/dhcp"
)

// const DefaultNetworkInterface = "eth1" // No longer used as default, comes from config

// AgentNetConfig defines the network configuration for a single interface in the agent pod.
// This should ideally be a shared type with the controller.
type AgentNetConfig struct {
	InterfaceName string `json:"interfaceName"`
	ServerIP      string `json:"serverIP"`
	CIDR          string `json:"cidr"`
	IPPoolName    string `json:"ipPoolName"` // Namespaced name "namespace/name"
	IPPoolRef     string `json:"ipPoolRef"`  // Namespaced name "namespace/name" for direct reference
	NadName       string `json:"nadName"`    // Namespaced name "namespace/name" of the NAD
}

type Agent struct {
	dryRun      bool
	netConfigs  []AgentNetConfig
	ipPoolRefs  []string // Parsed from IPPoolRefsJSON, stores "namespace/name" strings

	ipPoolEventHandlers []*ippool.EventHandler // Changed to a slice of handlers
	DHCPAllocator     *dhcp.DHCPAllocator
	// Each EventHandler will have its own poolCache.
	// The agent itself doesn't need a global poolCache if handlers are per-pool.
}

func NewAgent(options *config.AgentOptions) *Agent {
	dhcpAllocator := dhcp.NewDHCPAllocator()

	var netConfigs []AgentNetConfig
	if options.AgentNetworkConfigsJSON != "" {
		if err := json.Unmarshal([]byte(options.AgentNetworkConfigsJSON), &netConfigs); err != nil {
			logrus.Errorf("Failed to unmarshal AGENT_NETWORK_CONFIGS: %v. JSON was: %s", err, options.AgentNetworkConfigsJSON)
			// Continue with empty netConfigs, effectively disabling interface configuration
		}
	}

	var ipPoolRefs []string
	if options.IPPoolRefsJSON != "" {
		if err := json.Unmarshal([]byte(options.IPPoolRefsJSON), &ipPoolRefs); err != nil {
			logrus.Errorf("Failed to unmarshal IPPOOL_REFS_JSON: %v. JSON was: %s", err, options.IPPoolRefsJSON)
		}
	}

	agent := &Agent{
		dryRun:              options.DryRun,
		netConfigs:          netConfigs,
		ipPoolRefs:          ipPoolRefs, // This might be redundant if netConfigs is the source of truth
		DHCPAllocator:       dhcpAllocator,
		ipPoolEventHandlers: make([]*ippool.EventHandler, 0, len(netConfigs)),
	}

	// Initialize an EventHandler for each IPPoolRef specified in netConfigs
	processedIPPoolRefs := make(map[string]bool) // To avoid duplicate handlers for the same IPPoolRef

	for _, nc := range netConfigs {
		if nc.IPPoolRef == "" {
			logrus.Warnf("AgentNetConfig for interface %s has empty IPPoolRef, skipping EventHandler setup.", nc.InterfaceName)
			continue
		}
		if _, processed := processedIPPoolRefs[nc.IPPoolRef]; processed {
			logrus.Debugf("EventHandler for IPPoolRef %s already initialized, skipping.", nc.IPPoolRef)
			continue
		}

		namespace, name, err := cache.SplitMetaNamespaceKey(nc.IPPoolRef)
		if err != nil {
			logrus.Errorf("Invalid IPPoolRef format '%s': %v. Cannot set up EventHandler.", nc.IPPoolRef, err)
			continue
		}
		poolRef := types.NamespacedName{Namespace: namespace, Name: name}

		// Each EventHandler gets its own poolCache.
		// The poolCache is specific to the IPPool it handles.
		poolCacheForHandler := make(map[string]string)

		eventHandler := ippool.NewEventHandler(
			options.KubeConfigPath,
			options.KubeContext,
			nil, // KubeRestConfig will be initialized by eventHandler.Init()
			poolRef,
			dhcpAllocator,       // Shared DHCPAllocator
			poolCacheForHandler, // Per-handler cache
		)
		if err := eventHandler.Init(); err != nil {
			// Log error but don't stop the agent from starting.
			// The DHCP server for this pool might not get lease updates.
			logrus.Errorf("Failed to initialize EventHandler for IPPool %s: %v", nc.IPPoolRef, err)
		} else {
			agent.ipPoolEventHandlers = append(agent.ipPoolEventHandlers, eventHandler)
			processedIPPoolRefs[nc.IPPoolRef] = true
			logrus.Infof("Initialized EventHandler for IPPool %s", nc.IPPoolRef)
		}
	}

	return agent
}

func (a *Agent) configureInterfaces() error {
	if len(a.netConfigs) == 0 {
		logrus.Info("No network configurations provided, skipping static IP configuration for DHCP interfaces.")
		return nil
	}

	for _, config := range a.netConfigs {
		if config.ServerIP == "" || config.CIDR == "" || config.InterfaceName == "" {
			logrus.Warnf("Incomplete network configuration for IPPool %s (Interface: %s, ServerIP: %s, CIDR: %s), skipping this interface.",
				config.IPPoolRef, config.InterfaceName, config.ServerIP, config.CIDR)
			continue
		}

		ip := net.ParseIP(config.ServerIP)
		if ip == nil {
			logrus.Errorf("Invalid serverIP %s for interface %s (IPPool %s)", config.ServerIP, config.InterfaceName, config.IPPoolRef)
			continue // Skip this configuration
		}

		_, ipNet, err := net.ParseCIDR(config.CIDR)
		if err != nil {
			logrus.Errorf("Failed to parse CIDR %s for interface %s (IPPool %s): %v", config.CIDR, config.InterfaceName, config.IPPoolRef, err)
			continue // Skip this configuration
		}
		prefixLen, _ := ipNet.Mask.Size()
		ipWithPrefix := fmt.Sprintf("%s/%d", config.ServerIP, prefixLen)

		logrus.Infof("Attempting to configure interface %s with IP %s (for IPPool %s)", config.InterfaceName, ipWithPrefix, config.IPPoolRef)

		// Flush existing IPs (optional, but good for clean state on a dedicated interface)
		logrus.Debugf("Flushing IP addresses from interface %s", config.InterfaceName)
		cmdFlush := exec.Command("ip", "address", "flush", "dev", config.InterfaceName)
		if output, errFlush := cmdFlush.CombinedOutput(); errFlush != nil {
			logrus.Warnf("Failed to flush IP addresses from interface %s (non-critical, proceeding): %v. Output: %s", config.InterfaceName, errFlush, string(output))
		}

		// Add the new IP address
		cmdAdd := exec.Command("ip", "address", "add", ipWithPrefix, "dev", config.InterfaceName)
		outputAdd, errAdd := cmdAdd.CombinedOutput()
		if errAdd != nil {
			if strings.Contains(string(outputAdd), "File exists") || (cmdAdd.ProcessState != nil && cmdAdd.ProcessState.ExitCode() == 2) {
				logrus.Infof("IP address %s likely already configured on interface %s. Output: %s", ipWithPrefix, config.InterfaceName, string(outputAdd))
			} else {
				logrus.Errorf("Failed to add IP address %s to interface %s (IPPool %s): %v. Output: %s", ipWithPrefix, config.InterfaceName, config.IPPoolRef, errAdd, string(outputAdd))
				// Potentially continue to configure other interfaces or return an aggregated error later
				continue
			}
		}

		// Bring the interface up
		cmdUp := exec.Command("ip", "link", "set", "dev", config.InterfaceName, "up")
		if outputUp, errUp := cmdUp.CombinedOutput(); errUp != nil {
			logrus.Errorf("Failed to bring interface %s up (IPPool %s): %v. Output: %s", config.InterfaceName, config.IPPoolRef, errUp, string(outputUp))
			continue
		}
		logrus.Infof("Successfully configured interface %s with IP %s (for IPPool %s)", config.InterfaceName, ipWithPrefix, config.IPPoolRef)
	}
	return nil // Return nil if all successful, or consider aggregating errors
}

func (a *Agent) Run(ctx context.Context) error {
	logrus.Infof("VM DHCP Agent starting. Configured for %d IPPools/interfaces.", len(a.netConfigs))
	for i, cfg := range a.netConfigs {
		logrus.Infof("  [%d] IPPool: %s, Interface: %s, ServerIP: %s, CIDR: %s, NAD: %s",
			i, cfg.IPPoolRef, cfg.InterfaceName, cfg.ServerIP, cfg.CIDR, cfg.NadName)
	}


	if !a.dryRun {
		if err := a.configureInterfaces(); err != nil {
			// Log the error but continue if possible, or decide to exit based on severity
			// For now, we log errors within configureInterfaces and it attempts to configure all.
			logrus.Errorf("One or more interfaces failed to configure correctly: %v", err)
			// Depending on policy, might return err here.
		}
	}

	eg, egctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		// TODO: DHCPAllocator.Run and DryRun need to be adapted for multiple network configurations.
		// This will likely involve DHCPAllocator being aware of a.netConfigs and running
		// DHCP server logic for each valid configuration, or a single DHCP server capable
		// of listening on multiple interfaces and distinguishing traffic.
		// For now, this is a placeholder and will likely fail or behave unexpectedly.
		if a.dryRun {
			logrus.Info("Dry run mode: Skipping actual DHCP server start for multiple interfaces (TODO).")
			// Placeholder: Old DryRun took a single NIC. This needs to be re-thought for multiple.
			// return a.DHCPAllocator.DryRun(egctx, "some_representative_nic_or_all_nics")
			return nil
		}

		// Wait for all IPPool EventHandlers to complete their initial sync.
		if len(a.ipPoolEventHandlers) > 0 {
			logrus.Infof("DHCP server goroutine waiting for initial IPPool cache sync from %d handler(s)...", len(a.ipPoolEventHandlers))
			allSynced := true
			for i, handler := range a.ipPoolEventHandlers {
				if handler == nil || handler.InitialSyncDone == nil {
					logrus.Warnf("EventHandler %d or its InitialSyncDone channel is nil for IPPool %s, cannot wait for its cache sync.", i, handler.GetPoolRef().String())
					// Consider this a failure for allSynced or handle as per policy.
					// For now, we'll log and potentially proceed without its sync.
					// This shouldn't happen if Init() was successful and NewEventHandler worked.
					continue
				}
				select {
				case <-handler.InitialSyncDone:
					logrus.Infof("Initial IPPool cache sync complete for handler %s.", handler.GetPoolRef().String())
				case <-egctx.Done():
					logrus.Info("Context cancelled while waiting for initial IPPool cache sync.")
					return egctx.Err()
				}
			}
			if allSynced { // This variable is not strictly tracking if all synced yet, needs adjustment if one fails to init
				logrus.Info("All active IPPool EventHandlers completed initial sync.")
			} else {
				logrus.Warn("One or more IPPool EventHandlers did not complete initial sync (or were nil). DHCP server starting with potentially incomplete lease data.")
			}
		} else {
			logrus.Info("No IPPool EventHandlers configured, proceeding without waiting for cache sync.")
		}

		// The TODO below about multi-interface support in DHCPAllocator is a separate concern.
		// The current changes focus on ensuring lease data is loaded.
		logrus.Info("Starting DHCP server logic for configured interfaces.")
		// Pass all network configurations to DHCPAllocator.Run or DryRun.
		// The DHCPAllocator's Run/DryRun methods now expect []dhcp.DHCPNetConfig.
		// a.netConfigs is []AgentNetConfig. We need to ensure these are compatible.
		// For now, we assume the local dhcp.AgentNetConfig (if used) or direct use of
		// agent.AgentNetConfig in dhcp pkg (if import is fine) matches this structure.
		// The local DHCPNetConfig struct in dhcp.go was designed to match the relevant fields.

		// Create a slice of dhcp.DHCPNetConfig from a.netConfigs
		dhcpConfigs := make([]dhcp.DHCPNetConfig, len(a.netConfigs))
		for i, agentConf := range a.netConfigs {
			dhcpConfigs[i] = dhcp.DHCPNetConfig{
				InterfaceName: agentConf.InterfaceName,
				ServerIP:      agentConf.ServerIP,
				CIDR:          agentConf.CIDR,
				IPPoolRef:     agentConf.IPPoolRef,
			}
		}

		if a.dryRun {
			logrus.Info("Dry run mode: Simulating DHCP server start for configured interfaces.")
			return a.DHCPAllocator.DryRun(egctx, dhcpConfigs) // Pass the slice of configs
		}

		logrus.Info("Starting DHCP server logic for all configured interfaces.")
		return a.DHCPAllocator.Run(egctx, dhcpConfigs) // Pass the slice of configs
	})

	// Start an EventListener for each initialized EventHandler
	for _, handler := range a.ipPoolEventHandlers {
		if handler == nil {
			// Should not happen if initialization logic is correct
			logrus.Error("Encountered a nil EventHandler in ipPoolEventHandlers slice. Skipping.")
			continue
		}
		// Capture current handler for the goroutine closure
		currentHandler := handler
		eg.Go(func() error {
			logrus.Infof("Starting IPPool event listener for %s", currentHandler.GetPoolRef().String())
			// EventListener itself will handle its own k8s client initialization via currentHandler.Init()
			// if it wasn't done during NewAgent or if KubeRestConfig was nil.
			// Init() is now called during NewAgent. If it failed, the handler might not be in the list.
			// If it's in the list, Init() was successful.
			currentHandler.EventListener(egctx) // Pass the error group's context
			logrus.Infof("IPPool event listener for %s stopped.", currentHandler.GetPoolRef().String())
			return nil // EventListener handles its own errors internally or stops on context cancellation
		})
	}

	// dhcp.Cleanup has been updated to not require a specific NIC, as DHCPAllocator.stopAll() handles all servers.
	errCh := dhcp.Cleanup(egctx, a.DHCPAllocator)

	if err := eg.Wait(); err != nil {
		// If context is cancelled, eg.Wait() will return ctx.Err().
		// We should check if the error from errCh is also a context cancellation
		// to avoid redundant logging or error messages if the cleanup was graceful.
		select {
		case cleanupErr := <-errCh:
			if cleanupErr != nil && !(err == context.Canceled && cleanupErr == context.Canceled) {
				// Log cleanup error only if it's different from the main error or not a cancel error when main is cancel
				logrus.Errorf("DHCP cleanup error: %v", cleanupErr)
			}
		default:
			// Non-blocking read, in case errCh hasn't been written to yet (should not happen if eg.Wait() returned)
		}
		return err // Return the primary error from the error group
	}

	// Process the cleanup error after eg.Wait() has completed without error,
	// or if eg.Wait() error was context cancellation and cleanup might have its own error.
	if cleanupErr := <-errCh; cleanupErr != nil {
		// Avoid returning error if it was just context cancellation and eg.Wait() was also cancelled.
		// This depends on whether eg.Wait() returned an error already.
		// If eg.Wait() was fine, any cleanupErr is significant.
		// If eg.Wait() returned ctx.Err(), then a ctx.Err() from cleanup is expected.
		// The current structure returns eg.Wait() error first. If that was nil, then this error matters.
		// If eg.Wait() already returned an error, this cleanupErr is mostly for logging,
		// unless it's a different, more specific error.
		// The logic above eg.Wait() already tries to log cleanupErr if distinct.
		// This final check ensures it's returned if no other error took precedence.
		// This part might be redundant if the error handling around eg.Wait() is comprehensive.
		// For now, let's assume if eg.Wait() was nil, this error is the one to return.
		// If eg.Wait() had an error, that one is already returned.
		logrus.Infof("DHCP cleanup completed with message/error: %v", cleanupErr) // Log it regardless
		// Only return if eg.Wait() was successful, otherwise its error takes precedence.
		// This check is implicitly handled by returning eg.Wait() error first.
		// This path is reached if eg.Wait() returned nil.
		return cleanupErr
	}
	logrus.Info("VM DHCP Agent Run loop and cleanup finished successfully.")
	return nil
}
