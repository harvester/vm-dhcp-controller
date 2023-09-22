package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/joeyloman/kubevirt-ip-helper/pkg/app"

	log "github.com/sirupsen/logrus"
)

// https://github.com/kubevirt/client-go/blob/v0.59.0/examples/listvms/list-vms.go

var progname string = "kubevirt-ip-helper"

func init() {
	// Log as JSON instead of the default ASCII formatter.
	formatter := &log.TextFormatter{
		FullTimestamp: true,
	}
	log.SetFormatter(formatter)
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
}

func main() {
	log.Infof("(main) starting %s", progname)

	level, err := log.ParseLevel(os.Getenv("LOGLEVEL"))
	if err != nil {
		log.Warnf("(main) cannot determine loglevel, leaving it on Info")
	} else {
		log.Infof("(main) setting loglevel to %s", level)
		log.SetLevel(level)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())

	mainApp := app.Register(ctx)

	go func() {
		<-sig
		cancel()
		os.Exit(1)
	}()

	mainApp.Run()
	cancel()
}
