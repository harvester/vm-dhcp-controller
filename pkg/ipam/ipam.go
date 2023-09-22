package ipam

import (
	"fmt"
	"net"
	"net/netip"
	"sync"

	log "github.com/sirupsen/logrus"
)

type IPSubnet struct {
	cidr      netip.Prefix
	start     net.IP
	end       net.IP
	broadcast net.IP
	ips       map[string]bool
}

type IPAllocator struct {
	ipam  map[string]IPSubnet
	mutex sync.Mutex
}

func NewIPAllocator() *IPAllocator {
	ipam := make(map[string]IPSubnet)

	return &IPAllocator{
		ipam: ipam,
	}
}

func (a *IPAllocator) NewSubnet(name string, subnet string, start string, end string) (err error) {
	s := IPSubnet{}
	s.start = net.ParseIP(start)
	s.end = net.ParseIP(end)

	ipnet, err := netip.ParsePrefix(subnet)
	if err != nil {
		return err
	}
	s.cidr = ipnet

	startIP, err := netip.ParseAddr(start)
	if err != nil {
		return err
	}
	startIPCheck := ipnet.Contains(startIP)
	if !startIPCheck {
		return fmt.Errorf("start address %s is not within subnet %s range", start, subnet)
	}

	endIP, err := netip.ParseAddr(end)
	if err != nil {
		return err
	}
	endIPCheck := ipnet.Contains(endIP)
	if !endIPCheck {
		return fmt.Errorf("end address %s is not within subnet %s range", end, subnet)
	}

	startAddr, _ := netip.AddrFromSlice(s.start)
	endAddr, _ := netip.AddrFromSlice(s.end)
	if startAddr.Compare(endAddr) > 0 {
		return fmt.Errorf("end address %s is smaller then the start address %s", end, start)
	}

	subnetStart := net.IP(ipnet.Addr().AsSlice())
	subnetMask := net.CIDRMask(ipnet.Bits(), 32)
	subnetBroadcast := net.IP(make([]byte, 4))
	for i := range subnetStart {
		subnetBroadcast[i] = subnetStart[i] | ^subnetMask[i]
	}
	s.broadcast = subnetBroadcast

	if s.end.Equal(s.broadcast) {
		return fmt.Errorf("end address %s equals the broadcast address %s", s.end.String(), s.broadcast.String())
	}

	// pre-allocate all ips between the start and end address
	allocatedIPs := make(map[string]bool)
	for ip := startAddr; endAddr.Compare(ip.Prev()) > 0; ip = ip.Next() {
		allocatedIPs[ip.Unmap().String()] = false
	}
	s.ips = allocatedIPs

	a.ipam[name] = s

	return
}

func (a *IPAllocator) DeleteSubnet(name string) {
	delete(a.ipam, name)
}

func (a *IPAllocator) GetIP(name string, givenIP string) (string, error) {
	if _, exists := a.ipam[name]; !exists {
		return "", fmt.Errorf("network %s does not exists", name)
	}

	a.mutex.Lock()
	defer a.mutex.Unlock()

	if givenIP != "" {
		gIP, err := netip.ParseAddr(givenIP)
		if err != nil {
			return "", err
		}
		gIPCheck := a.ipam[name].cidr.Contains(gIP)
		if !gIPCheck {
			return "", fmt.Errorf("given ip %s is not cidr %s", givenIP, a.ipam[name].cidr)
		}

		if a.ipam[name].broadcast.Equal(gIP.Unmap().AsSlice()) {
			return "", fmt.Errorf("given ip %s equals the broadcast address %s", givenIP, a.ipam[name].broadcast.String())
		}
	}

	for ip, allocated := range a.ipam[name].ips {
		if givenIP != "" {
			if ip == givenIP {
				if allocated {
					return "", fmt.Errorf("given ip %s is already allocated", givenIP)
				} else {
					a.ipam[name].ips[ip] = true
					return ip, nil
				}
			}
		} else {
			if !allocated {
				a.ipam[name].ips[ip] = true
				return ip, nil
			}
		}
	}

	return "", fmt.Errorf("no more ips left in network %s", name)
}

func (a *IPAllocator) ReleaseIP(name string, givenIP string) (err error) {
	if _, exists := a.ipam[name]; !exists {
		return fmt.Errorf("network %s does not exists", name)
	}

	a.mutex.Lock()
	defer a.mutex.Unlock()

	if givenIP == "" {
		return fmt.Errorf("given ip is empty")
	}

	gIP, err := netip.ParseAddr(givenIP)
	if err != nil {
		return err
	}
	gIPCheck := a.ipam[name].cidr.Contains(gIP)
	if !gIPCheck {
		return fmt.Errorf("given ip %s is not cidr %s", givenIP, a.ipam[name].cidr)
	}

	for ip, allocated := range a.ipam[name].ips {
		if ip == givenIP {
			if allocated {
				a.ipam[name].ips[ip] = false
				return
			} else {
				return fmt.Errorf("given ip %s was not allocated", givenIP)
			}
		}
	}

	return fmt.Errorf("given ip %s not found in network %s", givenIP, name)
}

func (a *IPAllocator) Used(name string) (i int) {
	if _, exists := a.ipam[name]; !exists {
		log.Warnf("(ipam.Used) network %s does not exists", name)

		return
	}

	for _, allocated := range a.ipam[name].ips {
		if allocated {
			i++
		}
	}

	return i
}

func (a *IPAllocator) Available(name string) (i int) {
	if _, exists := a.ipam[name]; !exists {
		log.Warnf("(ipam.Available) network %s does not exists", name)

		return
	}

	for _, allocated := range a.ipam[name].ips {
		if allocated {
			i++
		}
	}

	return len(a.ipam[name].ips) - i
}

func (a *IPAllocator) Usage(name string) {
	if _, exists := a.ipam[name]; !exists {
		log.Warnf("(ipam.Usage) network %s does not exists", name)

		return
	}

	log.Infof("(ipam.Usage) %s: cidr=%s, start=%s, end=%s, broadcast=%s",
		name,
		a.ipam[name].cidr.String(),
		a.ipam[name].start.String(),
		a.ipam[name].end.String(),
		a.ipam[name].broadcast.String(),
	)

	var i int = 0
	log.Infof("(ipam.Usage) allocated ips:")
	for ip, allocated := range a.ipam[name].ips {
		if allocated {
			log.Infof("- %s", ip)
			i++
		}
	}

	log.Infof("(ipam.Usage) ipsinpool=%d, usedips=%d",
		len(a.ipam[name].ips),
		i,
	)
}

func New() *IPAllocator {
	return NewIPAllocator()
}
