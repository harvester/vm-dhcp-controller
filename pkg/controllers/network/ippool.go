package network

import (
	"context"

	"github.com/sirupsen/logrus"
	network "github.com/starbops/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	networkController "github.com/starbops/vm-dhcp-controller/pkg/generated/controllers/network.harvesterhci.io/v1alpha1"
)

type ipPoolHandler struct {
	ctx    context.Context
	ipPool networkController.IPPoolController
}

func RegisterIPPoolController(ctx context.Context, ipPool networkController.IPPoolController) {
	ipPoolHandler := &ipPoolHandler{
		ctx:    ctx,
		ipPool: ipPool,
	}

	ipPool.OnChange(ctx, "ippool-network-change", ipPoolHandler.OnIPPoolChange)
}

func (h *ipPoolHandler) OnIPPoolChange(key string, ipPool *network.IPPool) (*network.IPPool, error) {
	if ipPool == nil || ipPool.DeletionTimestamp != nil {
		return ipPool, nil
	}

	logrus.Infof("reoncilling ippool %s", key)
	return ipPool, nil
}
