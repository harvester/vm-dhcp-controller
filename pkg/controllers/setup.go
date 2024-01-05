package controllers

import (
	"github.com/starbops/vm-dhcp-controller/pkg/config"
	"github.com/starbops/vm-dhcp-controller/pkg/controllers/ippool"
	"github.com/starbops/vm-dhcp-controller/pkg/controllers/nad"
	"github.com/starbops/vm-dhcp-controller/pkg/controllers/vm"
	"github.com/starbops/vm-dhcp-controller/pkg/controllers/vmnetcfg"
)

type Config struct {
	Name string
}

var RegisterFuncList = []config.RegisterFunc{
	ippool.Register,
	nad.Register,
	vm.Register,
	vmnetcfg.Register,
}
