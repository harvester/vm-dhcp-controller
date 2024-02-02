package indexer

import (
	networkv1 "github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
)

const (
	VmNetCfgByNetworkIndex = "network.harvesterhci.io/vmnetcfg-by-network"
)

func VmNetCfgByNetwork(obj *networkv1.VirtualMachineNetworkConfig) ([]string, error) {
	ncs := obj.Spec.NetworkConfigs
	networkNames := make([]string, 0, len(ncs))
	for _, nc := range ncs {
		networkNames = append(networkNames, nc.NetworkName)
	}
	return networkNames, nil
}
