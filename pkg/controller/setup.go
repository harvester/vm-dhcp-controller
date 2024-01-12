package controller

import (
	"github.com/harvester/vm-dhcp-controller/pkg/config"
	"github.com/harvester/vm-dhcp-controller/pkg/controller/ippool"
	"github.com/harvester/vm-dhcp-controller/pkg/controller/vm"
	"github.com/harvester/vm-dhcp-controller/pkg/controller/vmnetcfg"
)

type Config struct {
	Name string
}

var RegisterFuncList = []config.RegisterFunc{
	ippool.Register,
	vm.Register,
	vmnetcfg.Register,
}
