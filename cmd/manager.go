package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/starbops/vm-dhcp-controller/pkg/config"
)

var (
	managerName string
	agentImage  string

	// managerCmd represents the manager command
	managerCmd = &cobra.Command{
		Use:   "manager",
		Short: "VM DHCP Controller Manager",
		Long: `VM DHCP Controller Manager

The manager collects following items:
- Cluster level bundle. Including resource manifests and pod logs.
- Any external bundles. e.g., Longhorn support bundle.

And it also waits for reports from support bundle agents. The reports contain:
- Logs of each node.`,
		Run: func(cmd *cobra.Command, args []string) {
			var image *config.Image
			imageTokens := strings.Split(agentImage, ":")
			if len(imageTokens) == 2 {
				image = config.NewImage(imageTokens[0], imageTokens[1])
			} else {
				fmt.Fprintf(os.Stderr, "Error parse agent image name\n")
				if err := cmd.Help(); err != nil {
					os.Exit(1)
				}
				os.Exit(0)
			}

			options := &config.Options{
				Name:       managerName,
				AgentImage: image,
			}

			if err := managerRun(options); err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err.Error())
				os.Exit(1)
			}
		},
	}
)

func init() {
	rootCmd.AddCommand(managerCmd)

	managerCmd.PersistentFlags().StringVar(&managerName, "name", os.Getenv("VM_DHCP_CONTROLLER_MANAGER_NAME"), "The name of the manager")
	managerCmd.PersistentFlags().StringVar(&agentImage, "image", "", "The container image for the spawned agents")
}
