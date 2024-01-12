package controller

import "github.com/harvester/vm-dhcp-controller/pkg/config"

const (
	ListAllPath = "/{networkName:.*}"
)

func NewRoutes(management *config.Management) []config.Route {
	return []config.Route{
		{
			Prefix: "/ipams",
			Handles: []config.Handle{
				{
					Allocator:           management.IPAllocator,
					Path:                ListAllPath,
					RegisterHandlerFunc: ListAllHandler,
				},
			},
		},
		{
			Prefix: "/caches",
			Handles: []config.Handle{
				{
					Allocator:           management.CacheAllocator,
					Path:                ListAllPath,
					RegisterHandlerFunc: ListAllHandler,
				},
			},
		},
	}
}
