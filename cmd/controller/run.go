package main

import (
	"context"
	"log"

	"github.com/rancher/wrangler/pkg/leader"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/harvester/vm-dhcp-controller/pkg/config"
	"github.com/harvester/vm-dhcp-controller/pkg/controller"
	"github.com/harvester/vm-dhcp-controller/pkg/server"
)

var (
	threadiness = 1
)

func run(options *config.ControllerOptions) error {
	logrus.Infof("Starting VM DHCP Controller: %s", name)

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

	management, err := config.SetupManagement(ctx, cfg, options)
	if err != nil {
		logrus.Fatalf("Error building controllers: %s", err.Error())
	}

	callback := func(ctx context.Context) {
		if err := management.Register(ctx, cfg, controller.RegisterFuncList); err != nil {
			panic(err)
		}

		if err := management.Start(threadiness); err != nil {
			panic(err)
		}

		<-ctx.Done()
	}

	httpServerOptions := config.HTTPServerOptions{
		DebugMode:        enableCacheDumpAPI,
		IPAllocator:      management.IPAllocator,
		CacheAllocator:   management.CacheAllocator,
		MetricsAllocator: management.MetricsAllocator,
	}
	s := server.NewHTTPServer(&httpServerOptions)
	s.RegisterControllerHandlers()

	eg, egctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		return s.Run()
	})

	eg.Go(func() error {
		<-egctx.Done()
		return s.Stop(egctx)
	})

	eg.Go(func() error {
		if noLeaderElection {
			callback(egctx)
		} else {
			leader.RunOrDie(egctx, "kube-system", "vm-dhcp-controllers", client, callback)
		}
		return nil
	})

	return eg.Wait()
}
