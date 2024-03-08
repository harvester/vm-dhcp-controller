package main

import (
	"fmt"
	"os"

	"github.com/rancher/wrangler/pkg/kubeconfig"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/harvester/vm-dhcp-controller/pkg/util"
	"github.com/harvester/webhook/pkg/config"
)

const defaultServiceCIDR = "10.53.0.0/16"

var (
	logDebug bool
	logTrace bool

	name        string
	serviceCIDR string
	options     config.Options
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "vm-dhcp-webhook",
	Short:   "VM DHCP Webhook",
	Long:    "VM DHCP Webhook",
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
		ctx := signals.SetupSignalContext()
		cfg, err := kubeconfig.GetNonInteractiveClientConfig(os.Getenv("KUBECONFIG")).ClientConfig()
		if err != nil {
			logrus.Fatal(err)
		}

		if err := run(ctx, cfg, &options); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	debug := util.EnvGetBool("VM_DHCP_WEBHOOK_DEBUG", false)
	trace := util.EnvGetBool("VM_DHCP_WEBHOOK_TRACE", false)

	rootCmd.PersistentFlags().BoolVar(&logDebug, "debug", debug, "set logging level to debug")
	rootCmd.PersistentFlags().BoolVar(&logTrace, "trace", trace, "set logging level to trace")

	rootCmd.Flags().StringVar(&name, "name", os.Getenv("VM_DHCP_AGENT_NAME"), "The name of the vm-dhcp-webhook instance")
	rootCmd.Flags().StringVar(&serviceCIDR, "service-cidr", defaultServiceCIDR, "")

	rootCmd.Flags().StringVar(&options.ControllerUsername, "controller-user", "harvester-vm-dhcp-controller", "The harvester controller username")
	rootCmd.Flags().StringVar(&options.GarbageCollectionUsername, "gc-user", "system:serviceaccount:kube-system:generic-garbage-collector", "The system username that performs garbage collection")
	rootCmd.Flags().StringVar(&options.Namespace, "namespace", os.Getenv("NAMESPACE"), "The harvester namespace")
	rootCmd.Flags().IntVar(&options.HTTPSListenPort, "https-port", 8443, "HTTPS listen port")
	rootCmd.Flags().IntVar(&options.Threadiness, "threadiness", 5, "Specify controller threads")

}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func execute() {
	cobra.CheckErr(rootCmd.Execute())
}
