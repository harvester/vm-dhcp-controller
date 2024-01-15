package agent

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/types"

	"github.com/harvester/vm-dhcp-controller/pkg/agent/ippool"
	"github.com/harvester/vm-dhcp-controller/pkg/config"
	"github.com/harvester/vm-dhcp-controller/pkg/dhcp"
)

const (
	defaultInterval         = 10 * time.Second
	defaultNetworkInterface = "eth1"
)

type Agent struct {
	ctx context.Context

	dryRun  bool
	poolRef types.NamespacedName

	ippoolEventHandler *ippool.EventHandler
	DHCPAllocator      *dhcp.DHCPAllocator
	poolCache          map[string]string
}

func NewAgent(ctx context.Context, options *config.AgentOptions) *Agent {
	dhcpAllocator := dhcp.NewDHCPAllocator()
	poolCache := make(map[string]string, 10)

	return &Agent{
		ctx: ctx,

		dryRun:  options.DryRun,
		poolRef: options.IPPoolRef,

		DHCPAllocator: dhcpAllocator,
		ippoolEventHandler: ippool.NewEventHandler(
			ctx,
			options.KubeConfigPath,
			options.KubeContext,
			nil,
			options.IPPoolRef,
			dhcpAllocator,
			poolCache,
		),
		poolCache: poolCache,
	}
}

func (a *Agent) Run() error {
	logrus.Infof("monitor ippool %s", a.poolRef.String())

	eg, egctx := errgroup.WithContext(a.ctx)

	eg.Go(func() error {
		select {
		case <-egctx.Done():
			return nil
		default:
			return a.DHCPAllocator.Run(defaultNetworkInterface, a.dryRun)
		}
	})

	eg.Go(func() error {
		select {
		case <-egctx.Done():
			return nil
		default:
			// initialize the ippoolEventListener handler
			if err := a.ippoolEventHandler.Init(); err != nil {
				logrus.Fatal(err)
			}
			return a.ippoolEventHandler.EventListener()
		}
	})

	if err := eg.Wait(); err != nil {
		logrus.Fatal(err)
	}

	return nil
}
