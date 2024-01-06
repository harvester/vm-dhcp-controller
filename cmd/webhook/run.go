package main

import (
	"context"

	"github.com/harvester/vm-dhcp-controller/pkg/webhook/ippool"
	"github.com/harvester/webhook/pkg/config"
	"github.com/harvester/webhook/pkg/server"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

func run(ctx context.Context, cfg *rest.Config, options *config.Options) error {
	logrus.Infof("Starting VM DHCP Webhook: %s", name)

	webhookServer := server.NewWebhookServer(ctx, cfg, name, options)

	if err := webhookServer.RegisterValidators(ippool.NewValidator()); err != nil {
		return err
	}

	if err := webhookServer.Start(); err != nil {
		return err
	}

	<-ctx.Done()

	return nil
}
