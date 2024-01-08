package main

import (
	"context"

	"github.com/harvester/webhook/pkg/config"
	"github.com/harvester/webhook/pkg/server"
	"github.com/rancher/wrangler/pkg/start"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"

	ctlcni "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/k8s.cni.cncf.io"
	ctlcniv1 "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/k8s.cni.cncf.io/v1"
	ctlkubevirt "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/kubevirt.io"
	ctlkubevirtv1 "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/kubevirt.io/v1"
	ctlnetwork "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/network.harvesterhci.io"
	ctlnetworkv1 "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/network.harvesterhci.io/v1alpha1"
	"github.com/harvester/vm-dhcp-controller/pkg/indexer"
	"github.com/harvester/vm-dhcp-controller/pkg/webhook/ippool"
)

type caches struct {
	nadCache      ctlcniv1.NetworkAttachmentDefinitionCache
	vmCache       ctlkubevirtv1.VirtualMachineCache
	vmnetcfgCache ctlnetworkv1.VirtualMachineNetworkConfigCache
}

func newCaches(ctx context.Context, cfg *rest.Config, threadiness int) (*caches, error) {
	var starters []start.Starter

	kubevirtFactory := ctlkubevirt.NewFactoryFromConfigOrDie(cfg)
	starters = append(starters, kubevirtFactory)

	cniFactory := ctlcni.NewFactoryFromConfigOrDie(cfg)
	starters = append(starters, cniFactory)

	networkFactory := ctlnetwork.NewFactoryFromConfigOrDie(cfg)
	starters = append(starters, networkFactory)

	// must declare cache before starting informers
	c := &caches{
		nadCache:      cniFactory.K8s().V1().NetworkAttachmentDefinition().Cache(),
		vmCache:       kubevirtFactory.Kubevirt().V1().VirtualMachine().Cache(),
		vmnetcfgCache: networkFactory.Network().V1alpha1().VirtualMachineNetworkConfig().Cache(),
	}

	// Indexer must be added before starting the informer, otherwise panic `cannot add indexers to running index` happens
	c.vmnetcfgCache.AddIndexer(indexer.VmNetCfgByNetworkIndex, indexer.VmNetCfgByNetwork)

	if err := start.All(ctx, threadiness, starters...); err != nil {
		return nil, err
	}

	return c, nil
}

func run(ctx context.Context, cfg *rest.Config, options *config.Options) error {
	logrus.Infof("Starting VM DHCP Webhook: %s", name)

	c, err := newCaches(ctx, cfg, options.Threadiness)
	if err != nil {
		return err
	}

	webhookServer := server.NewWebhookServer(ctx, cfg, name, options)

	if err := webhookServer.RegisterValidators(
		ippool.NewValidator(c.nadCache, c.vmnetcfgCache),
	); err != nil {
		return err
	}

	if err := webhookServer.Start(); err != nil {
		return err
	}

	<-ctx.Done()

	return nil
}
