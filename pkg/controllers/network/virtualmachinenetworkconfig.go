package network

import (
	"context"

	"github.com/sirupsen/logrus"
	network "github.com/starbops/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	networkController "github.com/starbops/vm-dhcp-controller/pkg/generated/controllers/network.harvesterhci.io/v1alpha1"
)

type virtualMachineNetworkConfigHandler struct {
	ctx                         context.Context
	virtualMachineNetworkConfig networkController.VirtualMachineNetworkConfigController
}

func RegisterVirtualMachineNetworkConfigController(ctx context.Context, virtualMachineNetworkConfig networkController.VirtualMachineNetworkConfigController) {
	virtualMachineNetworkConfigHandler := &virtualMachineNetworkConfigHandler{
		ctx:                         ctx,
		virtualMachineNetworkConfig: virtualMachineNetworkConfig,
	}

	virtualMachineNetworkConfig.OnChange(ctx, "virtualmachinenetworkconfig-network-change", virtualMachineNetworkConfigHandler.OnVirtualMachineNetworkConfigChange)
}

func (h *virtualMachineNetworkConfigHandler) OnVirtualMachineNetworkConfigChange(key string, virtualMachineNetworkConfig *network.VirtualMachineNetworkConfig) (*network.VirtualMachineNetworkConfig, error) {
	if virtualMachineNetworkConfig == nil || virtualMachineNetworkConfig.DeletionTimestamp != nil {
		return virtualMachineNetworkConfig, nil
	}

	logrus.Infof("reoncilling virtualmachinenetworkconfig %s", key)
	return virtualMachineNetworkConfig, nil
}
