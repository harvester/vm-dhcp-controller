package agent

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/starbops/vm-dhcp-controller/pkg/agent/ippool"
	"github.com/starbops/vm-dhcp-controller/pkg/config"
	"github.com/starbops/vm-dhcp-controller/pkg/dhcp"
	clientset "github.com/starbops/vm-dhcp-controller/pkg/generated/clientset/versioned"
	"github.com/starbops/vm-dhcp-controller/pkg/server"
)

const (
	defaultInterval         = 10 * time.Second
	defaultNetworkInterface = "eth1"
)

type Agent struct {
	ctx context.Context

	dryRun  bool
	poolRef types.NamespacedName

	k8sClient *clientset.Clientset

	ippoolEventHandler *ippool.EventHandler
	dhcpAllocator      *dhcp.DHCPAllocator
	poolCache          map[string]string
}

func NewK8sClient(kubeconfigPath string) *clientset.Clientset {
	var (
		config *rest.Config
		err    error
	)

	// creates the in-cluster config
	config, err = rest.InClusterConfig()
	if err != nil {
		if err == rest.ErrNotInCluster {
			// uses the current context in kubeconfig
			// path-to-kubeconfig -- for example, /root/.kube/config
			config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
			if err != nil {
				panic(err.Error())
			}
		} else {
			panic(err.Error())
		}
	}

	clientset, err := clientset.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	return clientset
}

func NewAgent(ctx context.Context, options *config.AgentOptions) *Agent {
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		homeDir := os.Getenv("HOME")
		kubeconfigPath = filepath.Join(homeDir, ".kube", "config")
	}

	kubeconfigContext := os.Getenv("KUBECONTEXT")

	dhcpAllocator := dhcp.NewDHCPAllocator()
	poolCache := make(map[string]string, 10)

	return &Agent{
		ctx: ctx,

		dryRun:    options.DryRun,
		k8sClient: NewK8sClient(options.KubeconfigPath),
		poolRef:   options.IPPoolRef,

		dhcpAllocator: dhcpAllocator,
		ippoolEventHandler: ippool.NewEventHandler(
			ctx,
			kubeconfigPath,
			kubeconfigContext,
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
			return a.dhcpAllocator.Run(defaultNetworkInterface, a.dryRun)
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

	eg.Go(func() error {
		select {
		case <-egctx.Done():
			return nil
		default:
			return server.NewServer(egctx)
		}
	})

	if err := eg.Wait(); err != nil {
		logrus.Fatal(err)
	}

	return nil
}
