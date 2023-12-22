//go:generate go run pkg/codegen/cleanup/main.go
//go:generate go run pkg/codegen/main.go

package main

import (
	"log"

	harvesterv1beta1 "github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	"github.com/rancher/wrangler/pkg/signals"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/starbops/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/starbops/vm-dhcp-controller/pkg/controllers"
	"github.com/starbops/vm-dhcp-controller/pkg/server"
)

func init() {
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		log.Fatal(err)
	}
	if err := harvesterv1beta1.AddToScheme(scheme); err != nil {
		log.Fatal(err)
	}
	if err := kubevirtv1.AddToScheme(scheme); err != nil {
		log.Fatal(err)
	}

}
func main() {

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
		return controllers.Start(egctx, config)
	})

	eg.Go(func() error {
		return server.NewServer(egctx)
	})

	err = eg.Wait()
	if err != nil {
		log.Fatal(err)
	}
}
