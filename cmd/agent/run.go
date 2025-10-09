package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/rancher/wrangler/v3/pkg/leader"
	"github.com/rancher/wrangler/v3/pkg/signals"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/harvester/vm-dhcp-controller/pkg/agent"
	"github.com/harvester/vm-dhcp-controller/pkg/config"
	"github.com/harvester/vm-dhcp-controller/pkg/server"
)

func run(options *config.AgentOptions) error {
	logrus.Infof("Starting VM DHCP Agent: %s", name)

	ctx := signals.SetupSignalContext()

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	cfg, err := kubeConfig.ClientConfig()
	if err != nil {
		log.Fatal(err)
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logrus.Fatalf("Error get client from kubeconfig: %s", err.Error())
	}

	agent := agent.NewAgent(options)

	callback := func(ctx context.Context) {
		if err := agent.Run(ctx); err != nil {
			// Check if the error is context.Canceled, which is expected on graceful shutdown.
			if errors.Is(err, context.Canceled) {
				logrus.Info("Agent run completed due to context cancellation.")
			} else {
				// For any other error, it's unexpected, so panic.
				logrus.Errorf("Agent run failed with unexpected error: %v", err)
				panic(err)
			}
		}
		// Wait for the context to be done, ensuring the leader election logic
		// holds the leadership until the context is fully cancelled.
		<-ctx.Done()
		logrus.Info("Leader election callback completed as context is done.")
	}

	httpServerOptions := config.HTTPServerOptions{
		DebugMode:     enableCacheDumpAPI,
		DHCPAllocator: agent.DHCPAllocator,
	}
	s := server.NewHTTPServer(&httpServerOptions)
	s.RegisterAgentHandlers()

	eg, egctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		return s.Run()
	})

	eg.Go(func() error {
		if noLeaderElection {
			callback(egctx)
		} else {
			podNamespace := os.Getenv("POD_NAMESPACE")
			if podNamespace == "" {
				logrus.Warn("POD_NAMESPACE environment variable not set, defaulting to 'default' for leader election. This might not be the desired namespace.")
				podNamespace = "default" // Fallback, though this should be set via Downward API
			}
			logrus.Infof("Using namespace %s for leader election", podNamespace)
			// TODO: use correct lock name
			leader.RunOrDie(egctx, podNamespace, "vm-dhcp-agents", client, callback)
		}
		return nil
	})

	errCh := server.Cleanup(egctx, s)

	if err := eg.Wait(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	// Return cleanup error message if any
	if err := <-errCh; err != nil {
		return err
	}

	logrus.Info("finished clean")

	return nil
}
