package main

import (
	"context"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/harvester/vm-dhcp-controller/pkg/agent"
	"github.com/harvester/vm-dhcp-controller/pkg/config"
	"github.com/harvester/vm-dhcp-controller/pkg/server"
)

func run(ctx context.Context, options *config.AgentOptions) error {
	logrus.Infof("Starting VM DHCP Agent: %s", name)

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

	eg.Go(func() error {
		<-egctx.Done()
		return s.Stop(egctx)
	})

	return eg.Wait()
}
