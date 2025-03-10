package main

import (
	"fmt"
	"os"
	"strings"

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
	noAgent                 bool
	enableCacheDumpAPI      bool
	agentNamespace          string
	agentImage              string
	agentServiceAccountName string
	noDHCP                  bool
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
		image, err := parseImageNameAndTag(agentImage)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		options := &config.ControllerOptions{
			NoAgent:                 noAgent,
			AgentNamespace:          agentNamespace,
			AgentImage:              image,
			AgentServiceAccountName: agentServiceAccountName,
			NoDHCP:                  noDHCP,
		}

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
	rootCmd.Flags().BoolVar(&noAgent, "no-agent", false, "Run vm-dhcp-controller without spawning agents")
	rootCmd.Flags().BoolVar(&enableCacheDumpAPI, "enable-cache-dump-api", false, "Enable cache dump APIs")
	rootCmd.Flags().BoolVar(&noDHCP, "no-dhcp", false, "Disable DHCP server on the spawned agents")
	rootCmd.Flags().StringVar(&agentNamespace, "namespace", os.Getenv("AGENT_NAMESPACE"), "The namespace for the spawned agents")
	rootCmd.Flags().StringVar(&agentImage, "image", os.Getenv("AGENT_IMAGE"), "The container image for the spawned agents")
	rootCmd.Flags().StringVar(&agentServiceAccountName, "service-account-name", os.Getenv("AGENT_SERVICE_ACCOUNT_NAME"), "The service account for the spawned agents")
}

// execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func parseImageNameAndTag(image string) (*config.Image, error) {
	idx := strings.LastIndex(image, ":")

	if idx == -1 {
		return config.NewImage(image, "latest"), nil
	}

	// If the last colon is immediately followed by the end of the string, it's invalid (no tag).
	if idx == len(image)-1 {
		return nil, fmt.Errorf("invalid image name: colon without tag")
	}

	if strings.Count(image, ":") > 2 {
		return nil, fmt.Errorf("invalid image name: multiple colons found")
	}

	if idx <= strings.LastIndex(image, "/") {
		return config.NewImage(image, "latest"), nil
	}

	return config.NewImage(image[:idx], image[idx+1:]), nil
}
