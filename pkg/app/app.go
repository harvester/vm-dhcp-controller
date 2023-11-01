package app

import (
	"context"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/joeyloman/kubevirt-ip-helper/pkg/cache"
	"github.com/joeyloman/kubevirt-ip-helper/pkg/controller/ippool"
	"github.com/joeyloman/kubevirt-ip-helper/pkg/controller/vm"
	"github.com/joeyloman/kubevirt-ip-helper/pkg/controller/vmnetcfg"
	"github.com/joeyloman/kubevirt-ip-helper/pkg/dhcp"
	"github.com/joeyloman/kubevirt-ip-helper/pkg/ipam"
)

type handler struct {
	ctx                  context.Context
	ipam                 *ipam.IPAllocator
	dhcp                 *dhcp.DHCPAllocator
	cache                *cache.CacheAllocator
	ippoolEventHandler   *ippool.EventHandler
	vmnetcfgEventHandler *vmnetcfg.EventHandler
	vmEventHandler       *vm.EventHandler
}

func Register(ctx context.Context) *handler {
	return &handler{
		ctx: ctx,
	}
}

func (h *handler) Run() {
	var kubeconfig_file string

	kubeconfig_file = os.Getenv("KUBECONFIG")
	if kubeconfig_file == "" {
		homedir := os.Getenv("HOME")
		kubeconfig_file = filepath.Join(homedir, ".kube", "config")
	}

	kubeconfig_context := os.Getenv("KUBECONTEXT")

	// initialize the ipam service
	h.ipam = ipam.New()

	// initialize the dhcp service
	h.dhcp = dhcp.New()

	// initialize the pool cache
	h.cache = cache.New()

	// initialize the ippoolEventListener handler
	h.ippoolEventHandler = ippool.NewEventHandler(
		h.ctx,
		h.ipam,
		h.dhcp,
		h.cache,
		kubeconfig_file,
		kubeconfig_context,
		nil,
		nil,
	)
	if err := h.ippoolEventHandler.Init(); err != nil {
		handleErr(err)
	}
	go h.ippoolEventHandler.EventListener()

	// give the ippool handler some time to gather all the pools and register the ipam subnets
	time.Sleep(time.Second * 10)

	// initialize the vmnetcfgEventListener handler
	h.vmnetcfgEventHandler = vmnetcfg.NewEventHandler(
		h.ctx,
		h.ipam,
		h.dhcp,
		h.cache,
		kubeconfig_file,
		kubeconfig_context,
		nil,
		nil,
	)
	if err := h.vmnetcfgEventHandler.Init(); err != nil {
		handleErr(err)
	}
	go h.vmnetcfgEventHandler.EventListener()

	// give the vmnetcfg handler some time to settle before collecting all the vms
	time.Sleep(time.Second * 30)

	// initialize the vmEventListener handler
	h.vmEventHandler = vm.NewEventHandler(
		h.ctx,
		h.ipam,
		h.dhcp,
		h.cache,
		kubeconfig_file,
		kubeconfig_context,
		nil,
		nil,
		nil,
	)
	if err := h.vmEventHandler.Init(); err != nil {
		handleErr(err)
	}
	go h.vmEventHandler.EventListener()

	// keep the main thread alive
	for {
		time.Sleep(time.Second)
	}
}

func handleErr(err error) {
	log.Panicf("(app.handleErr) %s", err.Error())
}
