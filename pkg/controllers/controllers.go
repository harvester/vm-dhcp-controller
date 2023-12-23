package controllers

import (
	"context"
	"time"

	kubevirt "github.com/harvester/harvester/pkg/generated/controllers/kubevirt.io"
	"github.com/rancher/lasso/pkg/cache"
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/pkg/start"
	sc "github.com/starbops/vm-dhcp-controller/pkg/controllers/network"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"

	"github.com/starbops/vm-dhcp-controller/pkg/crd"
	"github.com/starbops/vm-dhcp-controller/pkg/generated/controllers/network.harvesterhci.io"
)

const (
	baseDelay     = 5 * time.Millisecond
	maxDelay      = 5 * time.Minute
	defaultWorker = 5
)

func Register(ctx context.Context, restConfig *rest.Config) error {
	rateLimit := workqueue.NewItemExponentialFailureRateLimiter(baseDelay, maxDelay)
	workqueue.DefaultControllerRateLimiter()

	clientFactory, err := client.NewSharedClientFactory(restConfig, nil)
	if err != nil {
		return err
	}

	cacheFactory := cache.NewSharedCachedFactory(clientFactory, nil)
	scf := controller.NewSharedControllerFactory(cacheFactory, &controller.SharedControllerFactoryOptions{
		DefaultRateLimiter: rateLimit,
		DefaultWorkers:     defaultWorker,
	})

	networkFactory, err := network.NewFactoryFromConfigWithOptions(restConfig, &network.FactoryOptions{
		SharedControllerFactory: scf,
	})
	if err != nil {
		return err
	}

	// coreFactory, err := core.NewFactoryFromConfigWithOptions(restConfig, &core.FactoryOptions{
	// 	SharedControllerFactory: scf,
	// })
	// if err != nil {
	// 	return err
	// }

	// harvesterFactory, err := harvester.NewFactoryFromConfigWithOptions(restConfig, &harvester.FactoryOptions{
	// 	SharedControllerFactory: scf,
	// })
	// if err != nil {
	// 	return err
	// }

	kubevirtFactory, err := kubevirt.NewFactoryFromConfigWithOptions(restConfig, &kubevirt.FactoryOptions{
		SharedControllerFactory: scf,
	})
	if err != nil {
		return err
	}

	sc.RegisterIPPoolController(ctx, networkFactory.Network().V1alpha1().IPPool())
	sc.RegisterVirtualMachineController(ctx, kubevirtFactory.Kubevirt().V1().VirtualMachine())
	sc.RegisterVirtualMachineNetworkConfigController(ctx, networkFactory.Network().V1alpha1().VirtualMachineNetworkConfig())

	return start.All(ctx, 1, networkFactory)
}

func StartManager(ctx context.Context, restConfig *rest.Config) error {
	if err := crd.Create(ctx, restConfig); err != nil {
		return err
	}

	if err := Register(ctx, restConfig); err != nil {
		return err
	}

	<-ctx.Done()

	return nil
}

func StartAgent(ctx context.Context, restConfig *rest.Config) error {
	if err := crd.Create(ctx, restConfig); err != nil {
		return err
	}

	if err := Register(ctx, restConfig); err != nil {
		return err
	}

	<-ctx.Done()

	return nil
}
