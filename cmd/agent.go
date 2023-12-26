package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/starbops/vm-dhcp-controller/pkg/config"
	"k8s.io/apimachinery/pkg/types"
)

var (
	agentName     string
	dryRun        bool
	poolNamespace string
	poolName      string

	// agentCmd represents the agent command
	agentCmd = &cobra.Command{
		Use:   "agent",
		Short: "VM DHCP Controller Agent",
		Long: `VM DHCP Controller Agent

The agent collects following items:
- Cluster level bundle. Including resource manifests and pod logs.
- Any external bundles. e.g., Longhorn support bundle.

And it also waits for reports from support bundle agents. The reports contain:
- Logs of each node.`,
		Run: func(cmd *cobra.Command, args []string) {
			poolRef := types.NamespacedName{
				Namespace: poolNamespace,
				Name:      poolName,
			}

			options := &config.Options{
				Name:    agentName,
				PoolRef: poolRef,
			}
			if err := agentRun(options); err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err.Error())
				os.Exit(1)
			}
		},
	}
)

func init() {
	rootCmd.AddCommand(agentCmd)

	agentCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Run the agent without replying to any actual requests")
	agentCmd.PersistentFlags().StringVar(&agentName, "name", os.Getenv("VM_DHCP_CONTROLLER_AGENT_NAME"), "The name of the agent")
	agentCmd.PersistentFlags().StringVar(&poolNamespace, "pool-namespace", "", "The namespace of the pool that the agent should act upon")
	agentCmd.PersistentFlags().StringVar(&poolName, "pool-name", "", "The name of the pool that the agent should act upon")
}
