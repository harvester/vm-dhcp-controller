package agent

import (
	"github.com/harvester/vm-dhcp-controller/pkg/config"
)

const (
	ListAllPath = "/leases"
)

func NewRoutes(routeConfigs []config.RouteConfig) []config.Route {
	var routes = make([]config.Route, 0, 1)

	for _, rc := range routeConfigs {
		route := config.Route{
			Prefix: rc.Prefix,
			Handles: []config.Handle{
				{
					Allocator:           rc.Allocator,
					Path:                ListAllPath,
					RegisterHandlerFunc: ListAllHandler,
				},
			},
		}
		routes = append(routes, route)
	}

	return routes
}
