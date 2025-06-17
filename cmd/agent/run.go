package main

import (
	"errors"
	"net/http"

	"github.com/rancher/wrangler/v3/pkg/signals"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/harvester/vm-dhcp-controller/pkg/agent"
	"github.com/harvester/vm-dhcp-controller/pkg/config"
	"github.com/harvester/vm-dhcp-controller/pkg/server"
)

func run(options *config.AgentOptions) error {
	logrus.Infof("Starting VM DHCP Agent: %s", name)

	ctx := signals.SetupSignalContext()

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

	errCh := server.Cleanup(egctx, s)

	if err := eg.Wait(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	// Return cleanup error message if any
	if err := <-errCh; err != nil {
		return err
	}

	logrus.Info("finished clean")

	return nil
}
