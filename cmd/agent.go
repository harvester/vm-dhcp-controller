package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/starbops/vm-dhcp-controller/pkg/agent"
)

var (
	vdca = &agent.VMDHCPControllerAgent{}
)

// agentCmd represents the agent command
var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "VM DHCP Controller Agent",
	Long: `VM DHCP Controller Agent

The agent collects following items:
- Cluster level bundle. Including resource manifests and pod logs.
- Any external bundles. e.g., Longhorn support bundle.

And it also waits for reports from support bundle agents. The reports contain:
- Logs of each node.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := vdca.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(agentCmd)

	agentCmd.PersistentFlags().StringVar(&vdca.Name, "name", os.Getenv("VM_DHCP_CONTROLLER_AGENT_NAME"), "The VM DHCP controller agent name")
	agentCmd.PersistentFlags().BoolVar(&vdca.DryRun, "dry-run", false, "run the agent without replying to any actual requests")
}
