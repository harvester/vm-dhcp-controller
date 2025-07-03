package agent

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/types"

	"github.com/harvester/vm-dhcp-controller/pkg/agent/ippool"
	"github.com/harvester/vm-dhcp-controller/pkg/config"
	"github.com/harvester/vm-dhcp-controller/pkg/dhcp"
)

const DefaultNetworkInterface = "eth1"

type Agent struct {
	dryRun   bool
	nic      string
	poolRef  types.NamespacedName
	serverIP string // Static IP for the agent's DHCP interface
	cidr     string // CIDR for the ServerIP

	ippoolEventHandler *ippool.EventHandler
	DHCPAllocator      *dhcp.DHCPAllocator
	poolCache          map[string]string
}

func NewAgent(options *config.AgentOptions) *Agent {
	dhcpAllocator := dhcp.NewDHCPAllocator()
	poolCache := make(map[string]string, 10)

	return &Agent{
		dryRun:   options.DryRun,
		nic:      options.Nic,
		poolRef:  options.IPPoolRef,
		serverIP: options.ServerIP,
		cidr:     options.CIDR,

		DHCPAllocator: dhcpAllocator,
		ippoolEventHandler: ippool.NewEventHandler(
			options.KubeConfigPath,
			options.KubeContext,
			nil,
			options.IPPoolRef,
			dhcpAllocator,
			poolCache,
		),
		poolCache: poolCache,
	}
}

// ... (other imports remain the same)

func (a *Agent) configureInterface() error {
	if a.serverIP == "" || a.cidr == "" {
		logrus.Info("ServerIP or CIDR not provided, skipping static IP configuration for DHCP interface.")
		return nil
	}

	// Parse ServerIP to ensure it's a valid IP (primarily for logging/validation, ParseCIDR does more)
	ip := net.ParseIP(a.serverIP)
	if ip == nil {
		return fmt.Errorf("invalid serverIP: %s", a.serverIP)
	}

	// Parse CIDR to get IP and prefix. We primarily need the prefix length.
	// The IP from ParseCIDR might be different from a.serverIP if a.serverIP is not the network address.
	// We must use a.serverIP as the address to assign.
	_, ipNet, err := net.ParseCIDR(a.cidr)
	if err != nil {
		return fmt.Errorf("failed to parse CIDR %s: %w", a.cidr, err)
	}
	prefixLen, _ := ipNet.Mask.Size()

	ipWithPrefix := fmt.Sprintf("%s/%d", a.serverIP, prefixLen)

	logrus.Infof("Attempting to configure interface %s with IP %s", a.nic, ipWithPrefix)

	// Check if the IP is already configured (optional, but good for idempotency)
	// This is a bit complex to do robustly without external libraries for netlink,
	// so for now, we'll try to flush and add. A more robust solution would inspect existing IPs.

	// Flush existing IPs on the interface first to avoid conflicts (optional, can be dangerous if interface is shared)
	// For a dedicated Multus interface, this is usually safer.
	logrus.Debugf("Flushing IP addresses from interface %s", a.nic)
	cmdFlush := exec.Command("ip", "address", "flush", "dev", a.nic)
	if output, err := cmdFlush.CombinedOutput(); err != nil {
		logrus.Warnf("Failed to flush IP addresses from interface %s (non-critical, continuing): %v. Output: %s", a.nic, err, string(output))
		// Not returning error here as the add command might still work or be what's needed.
	}

	// Add the new IP address
	cmdAdd := exec.Command("ip", "address", "add", ipWithPrefix, "dev", a.nic)
	output, err := cmdAdd.CombinedOutput()
	if err != nil {
		// Check if the error is because the IP is already there (exit status 2 for `ip address add` often means this)
		// This is a heuristic and might not be universally true for all `ip` command versions or scenarios.
		if strings.Contains(string(output), "File exists") || (cmdAdd.ProcessState != nil && cmdAdd.ProcessState.ExitCode() == 2) {
			logrus.Infof("IP address %s likely already configured on interface %s. Output: %s", ipWithPrefix, a.nic, string(output))
			return nil // Assume already configured
		}
		return fmt.Errorf("failed to add IP address %s to interface %s: %w. Output: %s", ipWithPrefix, a.nic, err, string(output))
	}

	// Bring the interface up (it should be up from Multus, but good practice)
	cmdUp := exec.Command("ip", "link", "set", "dev", a.nic, "up")
	if output, err := cmdUp.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring interface %s up: %w. Output: %s", a.nic, err, string(output))
	}

	logrus.Infof("Successfully configured interface %s with IP %s", a.nic, ipWithPrefix)
	return nil
}


func (a *Agent) Run(ctx context.Context) error {
	logrus.Infof("monitor ippool %s", a.poolRef.String())

	if !a.dryRun { // Only configure interface if not in dry-run mode
		if err := a.configureInterface(); err != nil {
			// Depending on policy, either log and continue, or return error.
			// If DHCP server can't get its IP, it likely can't function.
			return fmt.Errorf("failed to configure DHCP server interface %s with static IP %s: %w", a.nic, a.serverIP, err)
		}
	}

	eg, egctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if a.dryRun {
			return a.DHCPAllocator.DryRun(egctx, a.nic)
		}
		// The DHCP server (go-dhcpd) needs to know its own IP (ServerIP from IPPool)
		// to correctly fill the 'siaddr' field (server IP address) in DHCP replies.
		// The current DHCPAllocator.Run does not take serverIP as an argument.
		// This might require modification to DHCPAllocator and go-dhcpd setup if it's not automatically using the interface's IP.
		// For now, we assume go-dhcpd will correctly use the IP set on `a.nic`.
		return a.DHCPAllocator.Run(egctx, a.nic)
	})

	eg.Go(func() error {
		if err := a.ippoolEventHandler.Init(); err != nil {
			return err
		}
		a.ippoolEventHandler.EventListener(egctx)
		return nil
	})

	errCh := dhcp.Cleanup(egctx, a.DHCPAllocator, a.nic)

	if err := eg.Wait(); err != nil {
		return err
	}

	// Return cleanup error message if any
	if err := <-errCh; err != nil {
		return err
	}

	return nil
}
