package util

import (
	"fmt"

	networkv1 "github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	ctlnetworkv1 "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/network.harvesterhci.io/v1alpha1"
	"github.com/harvester/vm-dhcp-controller/pkg/indexer"
)

type VmnetcfgGetter struct {
	VmnetcfgCache ctlnetworkv1.VirtualMachineNetworkConfigCache
}

// WhoUseIPPool requires adding network indexer to the vmnetcfg cache before invoking it
func (g *VmnetcfgGetter) WhoUseIPPool(ipPool *networkv1.IPPool) ([]*networkv1.VirtualMachineNetworkConfig, error) {
	networkName := fmt.Sprintf("%s/%s", ipPool.Namespace, ipPool.Name)
	vmnetcfgs, err := g.VmnetcfgCache.GetByIndex(indexer.VmNetCfgByNetworkIndex, networkName)
	if err != nil {
		return nil, err
	}

	return vmnetcfgs, nil
}
