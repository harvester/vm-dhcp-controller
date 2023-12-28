package agent

import (
	"github.com/starbops/vm-dhcp-controller/pkg/config"
	"github.com/starbops/vm-dhcp-controller/pkg/controllers/agent/ippool"
	"github.com/starbops/vm-dhcp-controller/pkg/controllers/agent/vmnetcfg"
)

type Config struct {
	Name     string
	PoolName string
	DryRun   bool
}

var RegisterFuncList = []config.RegisterFunc{
	ippool.Register,
	vmnetcfg.Register,
}
