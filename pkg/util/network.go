package util

import (
	"fmt"
	"net"
	"net/netip"

	networkv1 "github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
)

type PoolInfo struct {
	IPNet           *net.IPNet
	NetworkIPAddr   netip.Addr
	BroadcastIPAddr netip.Addr
	StartIPAddr     netip.Addr
	EndIPAddr       netip.Addr
	ServerIPAddr    netip.Addr
	RouterIPAddr    netip.Addr
}

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

func LoadPool(ipPool *networkv1.IPPool) (pi PoolInfo, err error) {
	pi.IPNet, pi.NetworkIPAddr, pi.BroadcastIPAddr, err = LoadCIDR(ipPool.Spec.IPv4Config.CIDR)
	if err != nil {
		return
	}

	if ipPool.Spec.IPv4Config.Pool.Start != "" {
		pi.StartIPAddr, err = netip.ParseAddr(ipPool.Spec.IPv4Config.Pool.Start)
		if err != nil {
			return
		}
	}

	if ipPool.Spec.IPv4Config.Pool.End != "" {
		pi.EndIPAddr, err = netip.ParseAddr(ipPool.Spec.IPv4Config.Pool.End)
		if err != nil {
			return
		}
	}

	if ipPool.Spec.IPv4Config.ServerIP != "" {
		pi.ServerIPAddr, err = netip.ParseAddr(ipPool.Spec.IPv4Config.ServerIP)
		if err != nil {
			return
		}
	}

	if ipPool.Spec.IPv4Config.Router != "" {
		pi.RouterIPAddr, err = netip.ParseAddr(ipPool.Spec.IPv4Config.Router)
		if err != nil {
			return
		}
	}

	return
}

func LoadAllocated(allocated map[string]string) (ipAddrList []netip.Addr) {
	for ip := range allocated {
		ipAddr, err := netip.ParseAddr(ip)
		if err != nil {
			continue
		}
		ipAddrList = append(ipAddrList, ipAddr)
	}
	return
}

func IsIPAddrInList(ipAddr netip.Addr, ipAddrList []netip.Addr) bool {
	for i := range ipAddrList {
		if ipAddr == ipAddrList[i] {
			return true
		}
	}
	return false
}

func IsIPInBetweenOf(ip, ip1, ip2 string) bool {
	ipAddr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	ip1Addr, err := netip.ParseAddr(ip1)
	if err != nil {
		return false
	}
	ip2Addr, err := netip.ParseAddr(ip2)
	if err != nil {
		return false
	}

	return ipAddr.Compare(ip1Addr) >= 0 && ipAddr.Compare(ip2Addr) <= 0
}