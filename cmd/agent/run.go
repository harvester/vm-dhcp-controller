package main

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/harvester/vm-dhcp-controller/pkg/agent"
	"github.com/harvester/vm-dhcp-controller/pkg/config"
	"github.com/harvester/vm-dhcp-controller/pkg/server"
	agentserver "github.com/harvester/vm-dhcp-controller/pkg/server/agent"
)

func run(ctx context.Context, options *config.AgentOptions) error {
	logrus.Infof("Starting VM DHCP Agent: %s", name)

	agent := agent.NewAgent(ctx, options)

	var routeConfigs = []config.RouteConfig{
		{
			Allocator: agent.DHCPAllocator,
		},
	}
	s := server.NewHTTPServer()
	s.Register(agentserver.NewRoutes(routeConfigs))
	go s.Run()

	return agent.Run()
}
