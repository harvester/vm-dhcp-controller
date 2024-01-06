package main

import (
	"context"

	"github.com/harvester/vm-dhcp-controller/pkg/agent"
	"github.com/harvester/vm-dhcp-controller/pkg/config"
	"github.com/sirupsen/logrus"
)

func run(ctx context.Context, options *config.AgentOptions) error {
	logrus.Infof("Starting VM DHCP Agent: %s", name)

	agent := agent.NewAgent(ctx, options)

	return agent.Run()
}
