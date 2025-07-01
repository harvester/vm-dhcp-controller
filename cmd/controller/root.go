package main

import (
	"fmt"
	"os"
	// "strings" // Removed as parseImageNameAndTag is removed

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/harvester/vm-dhcp-controller/pkg/config"
	"github.com/harvester/vm-dhcp-controller/pkg/util"
)

var (
	logDebug bool
	logTrace bool

	name                    string
	noLeaderElection        bool
	enableCacheDumpAPI      bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "vm-dhcp-controller",
	Short: "VM DHCP Controller",
	Long: `VM DHCP Controller

	The VM DHCP controller generates agents based on the IPPool objects
	defined in the cluster and coordinates the VirtualMachineNetworkConfig
	objects so that agents convert them into valid DHCP leases.
	`,
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
		options := &config.ControllerOptions{}

		if err := run(options); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	debug := util.EnvGetBool("VM_DHCP_CONTROLLER_DEBUG", false)
	trace := util.EnvGetBool("VM_DHCP_CONTROLLER_TRACE", false)

	rootCmd.PersistentFlags().BoolVar(&logDebug, "debug", debug, "set logging level to debug")
	rootCmd.PersistentFlags().BoolVar(&logTrace, "trace", trace, "set logging level to trace")

	rootCmd.Flags().StringVar(&name, "name", os.Getenv("VM_DHCP_CONTROLLER_NAME"), "The name of the vm-dhcp-controller instance")
	rootCmd.Flags().BoolVar(&noLeaderElection, "no-leader-election", false, "Run vm-dhcp-controller with leader-election disabled")
	rootCmd.Flags().BoolVar(&enableCacheDumpAPI, "enable-cache-dump-api", false, "Enable cache dump APIs")
}

// execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func execute() {
	cobra.CheckErr(rootCmd.Execute())
}

// func parseImageNameAndTag(image string) (*config.Image, error) { // Removed
