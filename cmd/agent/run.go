package main

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/starbops/vm-dhcp-controller/pkg/agent"
	"github.com/starbops/vm-dhcp-controller/pkg/config"
)

func Run(ctx context.Context, options *config.AgentOptions) error {
	logrus.Infof("Starting VM DHCP Agent: %s", name)

	agent := agent.NewAgent(ctx, options)

	return agent.Run()
}
