package agent

import (
	"log"

	"github.com/rancher/wrangler/pkg/signals"
	"github.com/starbops/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/starbops/vm-dhcp-controller/pkg/controllers"
	"github.com/starbops/vm-dhcp-controller/pkg/server"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
)

type VMDHCPControllerAgent struct {
	Name   string
	DryRun bool
}

func init() {
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		log.Fatal(err)
	}
}

func (a *VMDHCPControllerAgent) Run() error {
	ctx := signals.SetupSignalContext()

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		log.Fatal(err)
	}

	eg, egctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		return controllers.StartAgent(egctx, config)
	})

	eg.Go(func() error {
		return server.NewServer(egctx)
	})

	err = eg.Wait()
	if err != nil {
		log.Fatal(err)
	}

	return nil
}
