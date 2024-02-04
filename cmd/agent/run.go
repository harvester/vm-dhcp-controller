package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/harvester/vm-dhcp-controller/pkg/agent"
	"github.com/harvester/vm-dhcp-controller/pkg/config"
	"github.com/harvester/vm-dhcp-controller/pkg/server"
)

func run(ctx context.Context, options *config.AgentOptions) error {
	logrus.Infof("Starting VM DHCP Agent: %s", name)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	eg, egctx := errgroup.WithContext(ctx)

	agent := agent.NewAgent(options)

	httpServerOptions := config.HTTPServerOptions{
		DebugMode:     enableCacheDumpAPI,
		DHCPAllocator: agent.DHCPAllocator,
	}
	s := server.NewHTTPServer(&httpServerOptions)
	s.RegisterAgentHandlers()

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

	eg.Go(func() error {
		for {
			select {
			case sig := <-sigCh:
				logrus.Infof("received signal: %s", sig)
				cancel()
			case <-egctx.Done():
				return egctx.Err()
			}
		}
	})

	if err := eg.Wait(); err != nil {
		if errors.Is(err, context.Canceled) {
			logrus.Info("context canceled")
			return nil
		} else {
			return err
		}
	}

	logrus.Info("finished clean")

	return nil
}
