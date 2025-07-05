package main

import (
	"fmt"
	"os"

	// "github.com/rancher/wrangler/pkg/kv" // No longer needed for ippoolRef parsing
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	// "k8s.io/apimachinery/pkg/types" // No longer needed for IPPoolRef

	// "github.com/harvester/vm-dhcp-controller/pkg/agent" // DefaultNetworkInterface no longer used here
	"github.com/harvester/vm-dhcp-controller/pkg/config"
	"github.com/harvester/vm-dhcp-controller/pkg/util"
)

const (
	// Environment variable keys, must match controller-side
	agentNetworkConfigsEnvKey = "AGENT_NETWORK_CONFIGS"
	agentIPPoolRefsEnvKey     = "IPPOOL_REFS_JSON"
)

var (
	logDebug bool
	logTrace bool

	name               string
	dryRun             bool
	enableCacheDumpAPI bool
	kubeConfigPath     string
	kubeContext        string
	noLeaderElection   bool
	// Removed: nic, ippoolRef, serverIP, cidr
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "vm-dhcp-agent",
	Short:   "VM DHCP Agent",
	Long:    "VM DHCP Agent",
	Version: AppVersion,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		logrus.SetOutput(os.Stdout)
		if logDebug {
			logrus.SetLevel(logrus.DebugLevel)
		}
		if logTrace {
			logrus.SetLevel(logrus.TraceLevel)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		// Populate options from environment variables
		agentNetworkConfigsJSON := os.Getenv(agentNetworkConfigsEnvKey)
		ipPoolRefsJSON := os.Getenv(agentIPPoolRefsEnvKey)

		if agentNetworkConfigsJSON == "" {
			// Log a warning or error, as this is critical for the agent's function
			// Depending on desired behavior, could default to "[]" or exit.
			// For now, warn and proceed; the agent logic should handle empty/invalid JSON.
			logrus.Warnf("%s environment variable is not set or is empty. Agent may not configure any interfaces.", agentNetworkConfigsEnvKey)
			agentNetworkConfigsJSON = "[]" // Default to empty JSON array
		}

		if ipPoolRefsJSON == "" {
			logrus.Warnf("%s environment variable is not set or is empty.", agentIPPoolRefsEnvKey)
			ipPoolRefsJSON = "[]" // Default to empty JSON array
		}

		options := &config.AgentOptions{
			DryRun:                  dryRun,
			KubeConfigPath:          kubeConfigPath,
			KubeContext:             kubeContext,
			AgentNetworkConfigsJSON: agentNetworkConfigsJSON,
			IPPoolRefsJSON:          ipPoolRefsJSON,
		}

		if err := run(options); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	debug := util.EnvGetBool("VM_DHCP_AGENT_DEBUG", false)
	trace := util.EnvGetBool("VM_DHCP_AGENT_TRACE", false)

	rootCmd.PersistentFlags().BoolVar(&logDebug, "debug", debug, "set logging level to debug")
	rootCmd.PersistentFlags().BoolVar(&logTrace, "trace", trace, "set logging level to trace")

	rootCmd.Flags().StringVar(&name, "name", os.Getenv("VM_DHCP_AGENT_NAME"), "The name of the vm-dhcp-agent instance")
	rootCmd.Flags().StringVar(&kubeConfigPath, "kubeconfig", os.Getenv("KUBECONFIG"), "Path to the kubeconfig file")
	rootCmd.Flags().StringVar(&kubeContext, "kubecontext", os.Getenv("KUBECONTEXT"), "Context name")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Run vm-dhcp-agent without starting the DHCP server")
	rootCmd.Flags().BoolVar(&enableCacheDumpAPI, "enable-cache-dump-api", false, "Enable cache dump APIs")
	// Removed old flags that are now sourced from environment variables set by the controller:
	// - ippool-ref
	// - nic
	// - server-ip
	// - cidr
	rootCmd.Flags().BoolVar(&noLeaderElection, "no-leader-election", false, "Disable leader election")
}

// execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func execute() {
	cobra.CheckErr(rootCmd.Execute())
}
