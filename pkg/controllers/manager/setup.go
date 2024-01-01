package manager

import (
	"github.com/starbops/vm-dhcp-controller/pkg/config"
	"github.com/starbops/vm-dhcp-controller/pkg/controllers/manager/ippool"
	"github.com/starbops/vm-dhcp-controller/pkg/controllers/manager/nad"
)

type Config struct {
	Name string
}

var RegisterFuncList = []config.RegisterFunc{
	ippool.Register,
	nad.Register,
	// vm.Register,
}
