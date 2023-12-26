package cmd

import (
	"context"
	"log"

	"github.com/rancher/wrangler/pkg/leader"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/starbops/vm-dhcp-controller/pkg/config"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/starbops/vm-dhcp-controller/pkg/controllers/agent"
	"github.com/starbops/vm-dhcp-controller/pkg/controllers/manager"
)

var (
	threadiness = 1
)

func run(registerFuncList []config.RegisterFunc, leaderelection, createCRD bool, options *config.Options) error {
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
		klog.Fatalf("Error get client from kubeconfig: %s", err.Error())
	}

	management, err := config.SetupManagement(ctx, cfg, options)
	if err != nil {
		klog.Fatalf("Error building controllers: %s", err.Error())
	}

	callback := func(ctx context.Context) {
		if err := management.Register(ctx, cfg, createCRD, registerFuncList); err != nil {
			panic(err)
		}

		if err := management.Start(threadiness); err != nil {
			panic(err)
		}

		<-ctx.Done()
	}

	if leaderelection {
		leader.RunOrDie(ctx, "kube-system", "vm-dhcp-controllers", client, callback)
	} else {
		callback(ctx)
	}

	return nil
}

func managerRun(options *config.Options) error {
	klog.Infof("Starting VM DHCP Controller Manager: %s", managerName)

	return run(manager.RegisterFuncList, true, true, options)
}

func agentRun(options *config.Options) error {
	klog.Infof("Starting VM DHCP Controller Agent: %s", agentName)

	return run(agent.RegisterFuncList, false, false, options)
}
