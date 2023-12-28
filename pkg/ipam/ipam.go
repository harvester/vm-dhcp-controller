package ipam

import (
	"fmt"
	"net"
	"net/netip"
	"sync"

	"github.com/sirupsen/logrus"
)

var (
	UnspecifiedIPAddress = net.IP{0, 0, 0, 0}
)

type IPSubnet struct {
	ipNet     *net.IPNet
	start     net.IP
	end       net.IP
	broadcast net.IP
	ips       map[string]bool
}

type IPAllocator struct {
	ipam  map[string]IPSubnet
	mutex *sync.Mutex
}

func NewIPAllocator() *IPAllocator {
	return &IPAllocator{
		ipam:  make(map[string]IPSubnet),
		mutex: new(sync.Mutex),
	}
}

func (a *IPAllocator) NewIPSubnet(name string, cidr string, start, end net.IP) error {
	// Calculate the broadcast IP address
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}
	ipv4 := ip.To4()
	mask := ipNet.Mask
	broadcast := make(net.IP, 4)
	for i, octet := range ipv4 {
		broadcast[i] = octet | ^mask[i]
	}

	// Expand the map of allocated IP addresses ranging from the start to end IP address
	ips := make(map[string]bool)
	firstIP, ok := netip.AddrFromSlice(start)
	if !ok {
		return fmt.Errorf("cannot convert ip address %s", start)
	}
	lastIP, ok := netip.AddrFromSlice(end)
	if !ok {
		return fmt.Errorf("cannot convert ip address %s", end)
	}
	for ip := firstIP; lastIP.Compare(ip.Prev()) > 0; ip = ip.Next() {
		ips[ip.Unmap().String()] = false
	}

	ipSubnet := IPSubnet{
		ipNet:     ipNet,
		start:     start,
		end:       end,
		broadcast: broadcast,
		ips:       ips,
	}

	a.ipam[name] = ipSubnet

	return nil
}

func (a *IPAllocator) AllocateIP(name string, designatedIPStr string) (net.IP, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Sanity check
	if _, ok := a.ipam[name]; !ok {
		return UnspecifiedIPAddress, fmt.Errorf("network %s does not exist", name)
	}

	designatedIP := net.ParseIP(designatedIPStr)

	if designatedIP.String() != UnspecifiedIPAddress.String() {
		ok := a.ipam[name].ipNet.Contains(designatedIP)
		if !ok {
			return UnspecifiedIPAddress, fmt.Errorf("designated ip %s is not in the subnet %s/%s", designatedIP.String(), a.ipam[name].ipNet.IP.String(), a.ipam[name].ipNet.Mask.String())
		}

		if a.ipam[name].broadcast.Equal(designatedIP) {
			return UnspecifiedIPAddress, fmt.Errorf("designated ip %s equals the broadcast address %s", designatedIP.String(), a.ipam[name].broadcast.String())
		}
	}

	for ip, isAllocated := range a.ipam[name].ips {
		if designatedIP.String() != UnspecifiedIPAddress.String() {
			if ip == designatedIP.String() {
				if isAllocated {
					return UnspecifiedIPAddress, fmt.Errorf("designated ip %s is already allocated", designatedIP.String())
				} else {
					a.ipam[name].ips[ip] = true
					return net.ParseIP(ip), nil
				}
			}
		} else {
			if !isAllocated {
				a.ipam[name].ips[ip] = true
				return net.ParseIP(ip), nil
			}
		}
	}

	return UnspecifiedIPAddress, fmt.Errorf("no more ip addresses left in network %s", name)
}

func (a *IPAllocator) GetUsed(name string) (int, error) {
	var used int

	// Sanity check
	if _, ok := a.ipam[name]; !ok {
		return used, fmt.Errorf("network %s does not exist", name)
	}

	for _, isAllocated := range a.ipam[name].ips {
		if isAllocated {
			used++
		}
	}

	return used, nil
}

func (a *IPAllocator) GetAvailable(name string) (int, error) {
	var available int

	// Sanity check
	if _, ok := a.ipam[name]; !ok {
		return available, fmt.Errorf("network %s does not exist", name)
	}

	for _, isAllocated := range a.ipam[name].ips {
		if !isAllocated {
			available++
		}
	}

	return available, nil
}

func (a *IPAllocator) GetUsage(name string) error {
	// Sanity check
	if _, ok := a.ipam[name]; !ok {
		return fmt.Errorf("network %s does not exist", name)
	}

	logrus.Infof("ipam[%s] ipNet=%s/%s, start=%s, end=%s, broadcast=%s",
		name,
		a.ipam[name].ipNet.IP.String(),
		a.ipam[name].ipNet.Mask.String(),
		a.ipam[name].start.String(),
		a.ipam[name].end.String(),
		a.ipam[name].broadcast.String(),
	)

	var used int
	logrus.Infof("ipam[%s] allocatedIPs=", name)
	for ip, isAllocated := range a.ipam[name].ips {
		if isAllocated {
			logrus.Infof("ipam[%s] - %s", name, ip)
			used++
		}
	}

	logrus.Infof("ipam[%s] total=%d, in-use=%d, available=%d",
		name,
		len(a.ipam[name].ips),
		used,
		(len(a.ipam[name].ips) - used),
	)

	return nil
}
