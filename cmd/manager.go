package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/starbops/vm-dhcp-controller/pkg/manager"
)

var (
	vdcm = &manager.VMDHCPControllerManager{}
)

// managerCmd represents the manager command
var managerCmd = &cobra.Command{
	Use:   "manager",
	Short: "VM DHCP Controller Manager",
	Long: `VM DHCP Controller Manager

The manager collects following items:
- Cluster level bundle. Including resource manifests and pod logs.
- Any external bundles. e.g., Longhorn support bundle.

And it also waits for reports from support bundle agents. The reports contain:
- Logs of each node.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := vdcm.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(managerCmd)

	managerCmd.PersistentFlags().StringVar(&vdcm.Name, "name", os.Getenv("VM_DHCP_CONTROLLER_MANAGER_NAME"), "The VM DHCP controller manager name")
}
