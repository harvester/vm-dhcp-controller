package util

import (
	"fmt"
	"net"
	"net/netip"
)

func LoadCIDR(cidr string) (ipNet *net.IPNet, networkIPAddr netip.Addr, broadcastIPAddr netip.Addr, err error) {
	_, ipNet, err = net.ParseCIDR(cidr)
	if err != nil {
		return
	}

	networkIPAddr, ok := netip.AddrFromSlice(ipNet.IP)
	if !ok {
		err = fmt.Errorf("cannot convert ip address %s", ipNet.IP)
		return
	}

	broadcastIP := make(net.IP, len(ipNet.IP))
	copy(broadcastIP, ipNet.IP)
	for i := range broadcastIP {
		broadcastIP[i] |= ^ipNet.Mask[i]
	}
	broadcastIPAddr, ok = netip.AddrFromSlice(broadcastIP)
	if !ok {
		err = fmt.Errorf("cannot convert ip address %s", broadcastIP)
		return
	}

	return
}
