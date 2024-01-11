package controllers

import (
	"github.com/harvester/vm-dhcp-controller/pkg/config"
	"github.com/harvester/vm-dhcp-controller/pkg/controllers/ippool"
	"github.com/harvester/vm-dhcp-controller/pkg/controllers/vm"
	"github.com/harvester/vm-dhcp-controller/pkg/controllers/vmnetcfg"
)

type Config struct {
	Name string
}

var RegisterFuncList = []config.RegisterFunc{
	ippool.Register,
	vm.Register,
	vmnetcfg.Register,
}
