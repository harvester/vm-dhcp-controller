package main

import (
	"context"
	"errors"
	"net/http"

	"github.com/rancher/wrangler/pkg/leader"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/harvester/vm-dhcp-controller/pkg/agent"
	"github.com/harvester/vm-dhcp-controller/pkg/config"
	"github.com/harvester/vm-dhcp-controller/pkg/server"
)

func run(options *config.AgentOptions) error {
	logrus.Infof("Starting VM DHCP Agent: %s", name)

	ctx := signals.SetupSignalContext()

	cfg, err := buildRestConfig(options.KubeConfigPath, options.KubeContext)
	if err != nil {
		return err
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}

	callback := func(ctx context.Context) {
		agent := agent.NewAgent(options)

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
			return agent.Run(egctx)
		})

		errCh := server.Cleanup(egctx, s)

		if err := eg.Wait(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logrus.Errorf("agent runtime error: %v", err)
		}

		if err := <-errCh; err != nil {
			logrus.Errorf("cleanup error: %v", err)
		}
	}

	if noLeaderElection {
		callback(ctx)
		<-ctx.Done()
	} else {
		leader.RunOrDie(ctx, "kube-system", "vm-dhcp-agent-"+options.IPPoolRef.Name, client, callback)
	}

	logrus.Info("finished clean")

	return nil
}

func buildRestConfig(kubeconfig, kubecontext string) (*rest.Config, error) {
	if kubeconfig == "" {
		if cfg, err := rest.InClusterConfig(); err == nil {
			return cfg, nil
		}
	}

	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}
	overrides := &clientcmd.ConfigOverrides{}
	if kubecontext != "" {
		overrides.CurrentContext = kubecontext
	}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
}
