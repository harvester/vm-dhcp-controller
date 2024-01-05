package main

import (
	"context"
	"fmt"
	"os"

	"github.com/rancher/wrangler/pkg/kv"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/starbops/vm-dhcp-controller/pkg/config"
	"github.com/starbops/vm-dhcp-controller/pkg/utils"
	"k8s.io/apimachinery/pkg/types"
)

var (
	logDebug bool
	logTrace bool

	name           string
	dryRun         bool
	kubeconfigPath string
	ippoolRef      string
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
		ctx := context.Background()
		ipPoolNamespace, ipPoolName := kv.RSplit(ippoolRef, "/")
		options := &config.AgentOptions{
			DryRun:         dryRun,
			KubeconfigPath: kubeconfigPath,
			IPPoolRef: types.NamespacedName{
				Namespace: ipPoolNamespace,
				Name:      ipPoolName,
			},
		}

		if err := Run(ctx, options); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	debug := utils.EnvGetBool("VM_DHCP_AGENT_DEBUG", false)
	trace := utils.EnvGetBool("VM_DHCP_TRACE", false)

	rootCmd.PersistentFlags().BoolVar(&logDebug, "debug", debug, "set logging level to debug")
	rootCmd.PersistentFlags().BoolVar(&logTrace, "trace", trace, "set logging level to trace")

	rootCmd.Flags().StringVar(&name, "name", os.Getenv("VM_DHCP_AGENT_NAME"), "The name of the vm-dhcp-agent instance")
	rootCmd.Flags().StringVar(&kubeconfigPath, "kubeconfig", os.Getenv("KUBECONFIG"), "Path to the kubeconfig file")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Run vm-dhcp-agent without starting the DHCP server")
	rootCmd.Flags().StringVar(&ippoolRef, "ippool-ref", os.Getenv("IPPOOL_REF"), "The IPPool object the agent should sync with")
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}
