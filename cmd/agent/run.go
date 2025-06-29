package main

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/rancher/wrangler/pkg/leader"
	"github.com/rancher/wrangler/pkg/signals"
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
			panic(err)
		}
		<-ctx.Done()
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
			// TODO: use correct lock name
			leader.RunOrDie(egctx, "kube-system", "vm-dhcp-agents", client, callback)
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
