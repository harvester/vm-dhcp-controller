package agent

import (
	"context"
	"net"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/starbops/vm-dhcp-controller/pkg/config"
	"github.com/starbops/vm-dhcp-controller/pkg/dhcp"
	clientset "github.com/starbops/vm-dhcp-controller/pkg/generated/clientset/versioned"
	"github.com/starbops/vm-dhcp-controller/pkg/ipam"
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

	dhcpAllocator *dhcp.DHCPAllocator
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

func (a *Agent) sync() error {
	eg := errgroup.Group{}

	eg.Go(func() error {
		return a.syncLeases()
	})

	if err := eg.Wait(); err != nil {
		logrus.Fatal(err)
	}

	return nil
}

func (a *Agent) syncLeases() error {
	ticker := time.NewTicker(defaultInterval)

	for range ticker.C {
		ipPool, err := a.k8sClient.NetworkV1alpha1().IPPools(a.poolRef.Namespace).Get(a.ctx, a.poolRef.Name, v1.GetOptions{})
		if err != nil {
			return err
		}
		logrus.Infof("get ippool %s/%s", ipPool.Namespace, ipPool.Name)

		for ip, mac := range ipPool.Status.IPv4.Allocated {
			if mac == ipam.ExcludedMark {
				logrus.Infof("ip %s excluded", ip)
				continue
			}
			if a.dhcpAllocator.CheckLease(mac) {
				logrus.Infof("mac %s exists", mac)
				continue
			}
			if err := a.dhcpAllocator.AddLease(
				mac,
				ipPool.Spec.IPv4Config.ServerIP,
				net.ParseIP(ip),
				ipPool.Spec.IPv4Config.CIDR,
				ipPool.Spec.IPv4Config.Router,
				ipPool.Spec.IPv4Config.DNS,
				ipPool.Spec.IPv4Config.DomainName,
				ipPool.Spec.IPv4Config.DomainSearch,
				ipPool.Spec.IPv4Config.NTP,
				ipPool.Spec.IPv4Config.LeaseTime,
			); err != nil {
				return err
			}
			logrus.Infof("mac %s added", mac)
		}
	}

	return nil
}

func NewAgent(ctx context.Context, options *config.AgentOptions) *Agent {
	return &Agent{
		ctx: ctx,

		dryRun:    options.DryRun,
		k8sClient: NewK8sClient(options.KubeconfigPath),
		poolRef:   options.IPPoolRef,

		dhcpAllocator: dhcp.NewDHCPAllocator(),
	}
}

func (a *Agent) Run() error {
	eg, egctx := errgroup.WithContext(a.ctx)

	eg.Go(func() error {
		select {
		case <-egctx.Done():
			return nil
		default:
			return server.NewServer(egctx)
		}
	})

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
			return a.sync()
		}
	})

	if err := eg.Wait(); err != nil {
		logrus.Fatal(err)
	}

	return nil
}
