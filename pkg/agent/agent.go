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
	// "k8s.io/apimachinery/pkg/types" // No longer using types.NamespacedName directly here

	// "github.com/harvester/vm-dhcp-controller/pkg/agent/ippool" // Commented out for now
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

	// ippoolEventHandler *ippool.EventHandler // Commented out for now
	DHCPAllocator *dhcp.DHCPAllocator
	// poolCache          map[string]string // Commented out as it was tied to ippoolEventHandler
}

func NewAgent(options *config.AgentOptions) *Agent {
	dhcpAllocator := dhcp.NewDHCPAllocator()
	// poolCache := make(map[string]string, 10) // Commented out

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
		dryRun:     options.DryRun,
		netConfigs: netConfigs,
		ipPoolRefs: ipPoolRefs,

		DHCPAllocator: dhcpAllocator,
		// poolCache: poolCache, // Commented out
	}

	// Commenting out ippoolEventHandler initialization as its role needs re-evaluation
	// if agent.ippoolEventHandler = ippool.NewEventHandler(
	// 	options.KubeConfigPath,
	// 	options.KubeContext,
	// 	nil, // This was client, needs to be correctly passed if handler is used
	// 	types.NamespacedName{}, // This was a single IPPoolRef, needs to handle multiple or be re-thought
	// 	dhcpAllocator,
	// 	poolCache,
	// );

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

		// Commenting out ippoolEventHandler related sync logic
		/*
			if a.ippoolEventHandler != nil && a.ippoolEventHandler.InitialSyncDone != nil {
				logrus.Info("DHCP server goroutine waiting for initial IPPool cache sync...")
				select {
				case <-a.ippoolEventHandler.InitialSyncDone:
					logrus.Info("Initial IPPool cache sync complete. Starting DHCP server.")
				case <-egctx.Done():
					logrus.Info("Context cancelled while waiting for initial IPPool cache sync.")
					return egctx.Err()
				}
			} else {
				logrus.Warn("ippoolEventHandler or InitialSyncDone channel is nil, cannot wait for cache sync.")
			}
		*/
		logrus.Info("Starting DHCP server logic for configured interfaces (TODO: Needs multi-interface support in DHCPAllocator).")
		return a.DHCPAllocator.Run(egctx, a.netConfigs) // Placeholder: Pass all configs
	})

	// Commenting out ippoolEventHandler logic as its role is unclear in the new multi-pool agent model
	/*
		eg.Go(func() error {
			if a.ippoolEventHandler == nil {
				logrus.Info("ippoolEventHandler is not initialized, skipping event listener.")
				return nil
			}
			if err := a.ippoolEventHandler.Init(); err != nil {
				return err
			}
			a.ippoolEventHandler.EventListener(egctx)
			return nil
		})
	*/

	// TODO: dhcp.Cleanup needs to be adapted for multiple NICs/configs if it does interface-specific cleanup.
	// For now, passing an empty string or the first interface name as a placeholder.
	// The actual cleanup logic inside dhcp.Cleanup will need to be aware of all managed interfaces.
	firstNic := ""
	if len(a.netConfigs) > 0 {
		firstNic = a.netConfigs[0].InterfaceName
	}
	errCh := dhcp.Cleanup(egctx, a.DHCPAllocator, firstNic) // Placeholder for NIC

	if err := eg.Wait(); err != nil {
		return err
	}

	// Return cleanup error message if any
	if err := <-errCh; err != nil {
		return err
	}

	return nil
}
