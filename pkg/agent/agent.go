package agent

import (
	"context"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/types"

	"github.com/harvester/vm-dhcp-controller/pkg/agent/ippool"
	"github.com/harvester/vm-dhcp-controller/pkg/config"
	"github.com/harvester/vm-dhcp-controller/pkg/dhcp"
)

const DefaultNetworkInterface = "eth1"

type Agent struct {
	dryRun  bool
	nic     string
	poolRef types.NamespacedName

	ippoolEventHandler *ippool.EventHandler
	DHCPAllocator      *dhcp.DHCPAllocator
	poolCache          map[string]string
}

func NewAgent(options *config.AgentOptions) *Agent {
	dhcpAllocator := dhcp.NewDHCPAllocator()
	poolCache := make(map[string]string, 10)

	return &Agent{
		dryRun:  options.DryRun,
		nic:     options.Nic,
		poolRef: options.IPPoolRef,

		DHCPAllocator: dhcpAllocator,
		ippoolEventHandler: ippool.NewEventHandler(
			options.KubeConfigPath,
			options.KubeContext,
			nil,
			options.IPPoolRef,
			dhcpAllocator,
			poolCache,
			options.Nic,
		),
		poolCache: poolCache,
	}
}

func (a *Agent) Run(ctx context.Context) error {
	logrus.Infof("monitor ippool %s", a.poolRef.String())

	eg, egctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if a.dryRun {
			return a.DHCPAllocator.DryRun(egctx, a.nic)
		}
		return a.DHCPAllocator.Run(egctx, a.nic)
	})

	eg.Go(func() error {
		if err := a.ippoolEventHandler.Init(); err != nil {
			return err
		}
		a.ippoolEventHandler.EventListener(egctx)
		return nil
	})

	errCh := dhcp.Cleanup(egctx, a.DHCPAllocator, a.nic)

	if err := eg.Wait(); err != nil {
		return err
	}

	// Return cleanup error message if any
	if err := <-errCh; err != nil {
		return err
	}

	return nil
}
