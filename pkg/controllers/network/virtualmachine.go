package network

import (
	"context"

	kubevirtv1 "github.com/harvester/harvester/pkg/generated/controllers/kubevirt.io/v1"
	"github.com/sirupsen/logrus"
	kubevirt "kubevirt.io/api/core/v1"
)

type virtualMachineHandler struct {
	ctx      context.Context
	kubevirt kubevirtv1.VirtualMachineController
}

func RegisterVirtualMachineController(ctx context.Context, kubevirt kubevirtv1.VirtualMachineController) {
	virtualMachineHandler := &virtualMachineHandler{
		ctx:      ctx,
		kubevirt: kubevirt,
	}

	kubevirt.OnChange(ctx, "ippool-network-change", virtualMachineHandler.OnVirtualMachineChange)
}

func (h *virtualMachineHandler) OnVirtualMachineChange(key string, virtualMachine *kubevirt.VirtualMachine) (*kubevirt.VirtualMachine, error) {
	if virtualMachine == nil || virtualMachine.DeletionTimestamp != nil {
		return virtualMachine, nil
	}

	logrus.Infof("reoncilling virtualmachine %s", key)
	return virtualMachine, nil
}
