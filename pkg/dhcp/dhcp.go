package dhcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	"github.com/insomniacslk/dhcp/rfc1035label"
)

type DHCPLease struct {
	ServerIP     net.IP
	ClientIP     net.IP
	SubnetMask   net.IPMask
	Router       net.IP
	DNS          []net.IP
	DomainName   string
	DomainSearch []string
	NTP          []net.IP
	LeaseTime    int
}

func (l *DHCPLease) String() string {
	b, err := json.Marshal(l)
	if err != nil {
		return ""
	}
	return string(b)
}

type DHCPAllocator struct {
	leases  map[string]DHCPLease
	servers map[string]*server4.Server
	mutex   sync.RWMutex
}

func New() *DHCPAllocator {
	return NewDHCPAllocator()
}

func NewDHCPAllocator() *DHCPAllocator {
	leases := make(map[string]DHCPLease)
	servers := make(map[string]*server4.Server)

	return &DHCPAllocator{
		leases:  leases,
		servers: servers,
	}
}

func (a *DHCPAllocator) AddLease(
	hwAddr string,
	serverIP string,
	clientIP string,
	cidr string,
	routerIP string,
	dnsServers []string,
	domainName *string,
	domainSearch []string,
	ntpServers []string,
	leaseTime *int,
) (err error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if hwAddr == "" {
		return fmt.Errorf("hwaddr is empty")
	}

	if _, err := net.ParseMAC(hwAddr); err != nil {
		return fmt.Errorf("hwaddr %s is not valid", hwAddr)
	}

	if a.checkLease(hwAddr) {
		return fmt.Errorf("lease for hwaddr %s already exists", hwAddr)
	}

	lease := DHCPLease{}
	lease.ServerIP = net.ParseIP(serverIP)
	lease.ClientIP = net.ParseIP(clientIP)

	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}
	lease.SubnetMask = ipNet.Mask

	lease.Router = net.ParseIP(routerIP)
	for _, dnsServer := range dnsServers {
		dnsServerIP := net.ParseIP(dnsServer)
		lease.DNS = append(lease.DNS, dnsServerIP)
	}
	if domainName == nil {
		lease.DomainName = ""
	} else {
		lease.DomainName = *domainName
	}
	lease.DomainSearch = domainSearch

	for _, ntpServer := range ntpServers {
		ntpServerIP := net.ParseIP(ntpServer)
		if ntpServerIP.To4() != nil {
			lease.NTP = append(lease.NTP, ntpServerIP.To4())
		} else {
			ntpServerIPs, err := net.LookupIP(ntpServer)
			if err != nil {
				logrus.Errorf("(dhcp.AddLease) cannot get any ip addresses from ntp domainname entry %s: %s", ntpServer, err)
			}
			for _, ip := range ntpServerIPs {
				if ip.To4() != nil {
					lease.NTP = append(lease.NTP, ip.To4())
				}
			}
		}
	}

	if leaseTime == nil {
		lease.LeaseTime = 0
	} else {
		lease.LeaseTime = *leaseTime
	}

	a.leases[hwAddr] = lease

	logrus.Infof("(dhcp.AddLease) lease added for hardware address: %s", hwAddr)

	return
}

func (a *DHCPAllocator) checkLease(hwAddr string) bool {
	_, exists := a.leases[hwAddr]

	return exists
}

func (a *DHCPAllocator) GetLease(hwAddr string) (lease DHCPLease) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	return a.leases[hwAddr]
}

func (a *DHCPAllocator) DeleteLease(hwAddr string) (err error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if !a.checkLease(hwAddr) {
		return fmt.Errorf("lease for hwaddr %s does not exists", hwAddr)
	}

	delete(a.leases, hwAddr)

	logrus.Infof("(dhcp.DeleteLease) lease deleted for hardware address: %s", hwAddr)

	return
}

func (a *DHCPAllocator) Usage() {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	for hwaddr, lease := range a.leases {
		logrus.Infof("(dhcp.Usage) lease: hwaddr=%s, clientip=%s, netmask=%s, router=%s, dns=%+v, domain=%s, domainsearch=%+v, ntp=%+v, leasetime=%d",
			hwaddr,
			lease.ClientIP.String(),
			lease.SubnetMask.String(),
			lease.Router.String(),
			lease.DNS,
			lease.DomainName,
			lease.DomainSearch,
			lease.NTP,
			lease.LeaseTime,
		)
	}
}

func (a *DHCPAllocator) dhcpHandler(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	if m == nil {
		logrus.Errorf("(dhcp.dhcpHandler) packet is nil!")
		return
	}

	logrus.Tracef("(dhcp.dhcpHandler) INCOMING PACKET=%s", m.Summary())

	if m.OpCode != dhcpv4.OpcodeBootRequest {
		logrus.Errorf("(dhcp.dhcpHandler) not a BootRequest!")
		return
	}

	reply, err := dhcpv4.NewReplyFromRequest(m)
	if err != nil {
		logrus.Errorf("(dhcp.dhcpHandler) NewReplyFromRequest failed: %v", err)
		return
	}

	lease := a.leases[m.ClientHWAddr.String()]

	if lease.ClientIP == nil {
		logrus.Warnf("(dhcp.dhcpHandler) NO LEASE FOUND: hwaddr=%s", m.ClientHWAddr.String())

		return
	}

	logrus.Debugf("(dhcp.dhcpHandler) LEASE FOUND: hwaddr=%s, serverip=%s, clientip=%s, mask=%s, router=%s, dns=%+v, domainname=%s, domainsearch=%+v, ntp=%+v, leasetime=%d",
		m.ClientHWAddr.String(),
		lease.ServerIP.String(),
		lease.ClientIP.String(),
		lease.SubnetMask.String(),
		lease.Router.String(),
		lease.DNS,
		lease.DomainName,
		lease.DomainSearch,
		lease.NTP,
		lease.LeaseTime,
	)

	reply.ClientIPAddr = lease.ClientIP
	reply.ServerIPAddr = lease.ServerIP
	reply.YourIPAddr = lease.ClientIP
	reply.TransactionID = m.TransactionID
	reply.ClientHWAddr = m.ClientHWAddr
	reply.Flags = m.Flags
	reply.GatewayIPAddr = m.GatewayIPAddr

	reply.UpdateOption(dhcpv4.OptServerIdentifier(lease.ServerIP))
	reply.UpdateOption(dhcpv4.OptSubnetMask(lease.SubnetMask))
	reply.UpdateOption(dhcpv4.OptRouter(lease.Router))

	if len(lease.DNS) > 0 {
		reply.UpdateOption(dhcpv4.OptDNS(lease.DNS...))
	}

	if lease.DomainName != "" {
		reply.UpdateOption(dhcpv4.OptDomainName(lease.DomainName))
	}

	if len(lease.DomainSearch) > 0 {
		dsl := rfc1035label.NewLabels()
		dsl.Labels = append(dsl.Labels, lease.DomainSearch...)

		reply.UpdateOption(dhcpv4.OptDomainSearch(dsl))
	}

	if len(lease.NTP) > 0 {
		reply.UpdateOption(dhcpv4.OptNTPServers(lease.NTP...))
	}

	if lease.LeaseTime > 0 {
		reply.UpdateOption(dhcpv4.OptIPAddressLeaseTime(time.Duration(lease.LeaseTime) * time.Second))
	} else {
		// default lease time: 1 year
		reply.UpdateOption(dhcpv4.OptIPAddressLeaseTime(31536000 * time.Second))
	}

	switch messageType := m.MessageType(); messageType {
	case dhcpv4.MessageTypeDiscover:
		logrus.Debugf("(dhcp.dhcpHandler) DHCPDISCOVER: %+v", m)
		reply.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeOffer))
		logrus.Debugf("(dhcp.dhcpHandler) DHCPOFFER: %+v", reply)
	case dhcpv4.MessageTypeRequest:
		logrus.Debugf("(dhcp.dhcpHandler) DHCPREQUEST: %+v", m)
		reply.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeAck))
		logrus.Debugf("(dhcp.dhcpHandler) DHCPACK: %+v", reply)
	default:
		logrus.Warnf("(dhcp.dhcpHandler) Unhandled message type for hwaddr [%s]: %v", m.ClientHWAddr.String(), messageType)
		return
	}

	if _, err := conn.WriteTo(reply.ToBytes(), peer); err != nil {
		logrus.Errorf("(dhcp.dhcpHandler) Cannot reply to client: %v", err)
	}
}

func (a *DHCPAllocator) Run(ctx context.Context, nic string) (err error) {
	logrus.Infof("(dhcp.Run) starting DHCP service on nic %s", nic)

	var server *server4.Server

	// we need to listen on 0.0.0.0 otherwise client discovers will not be answered
	laddr := net.UDPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: 67,
	}

	server, err = server4.NewServer(nic, &laddr, a.dhcpHandler)
	if err != nil {
		return
	}

	go func() {
		if err := server.Serve(); err != nil {
			logrus.Errorf("(dhcp.Run) DHCP server on nic %s exited with error: %v", nic, err)
		}
	}()

	a.servers[nic] = server

	return nil
}

func (a *DHCPAllocator) DryRun(ctx context.Context, nic string) (err error) {
	logrus.Infof("(dhcp.DryRun) starting DHCP service on nic %s", nic)

	var server *server4.Server

	a.servers[nic] = server

	return nil
}

func (a *DHCPAllocator) stop(nic string) (err error) {
	logrus.Infof("(dhcp.Stop) stopping DHCP service on nic %s", nic)

	if a.servers[nic] == nil {
		return nil
	}

	return a.servers[nic].Close()
}

func (a *DHCPAllocator) ListAll(name string) (map[string]string, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	leases := make(map[string]string, len(a.leases))
	for mac, lease := range a.leases {
		leases[mac] = lease.String()
	}

	return leases, nil
}

func Cleanup(ctx context.Context, a *DHCPAllocator, nic string) <-chan error {
	errCh := make(chan error)

	go func() {
		<-ctx.Done()
		defer close(errCh)

		errCh <- a.stop(nic)
	}()

	return errCh
}
