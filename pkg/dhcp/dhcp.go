package dhcp

import (
	"fmt"
	"net"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
)

type DHCPLease struct {
	ServerIP   net.IP
	ClientIP   net.IP
	SubnetMask net.IPMask
	Router     net.IP
	DNS        []net.IP
	Reference  string
}

type DHCPAllocator struct {
	leases  map[string]DHCPLease
	servers map[string]*server4.Server
	mutex   sync.Mutex
}

func NewDHCPAllocator() *DHCPAllocator {
	leases := make(map[string]DHCPLease)
	servers := make(map[string]*server4.Server)

	return &DHCPAllocator{
		leases:  leases,
		servers: servers,
	}
}

func (a *DHCPAllocator) AddLease(hwAddr string, serverIP string, clientIP string, subnetMask string, routerIP string, DNSServers []string, ref string) (err error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if a.CheckLease(hwAddr) {
		return fmt.Errorf("lease for hwaddr %s already exists", hwAddr)
	}

	lease := DHCPLease{}
	lease.ServerIP = net.ParseIP(serverIP)
	lease.ClientIP = net.ParseIP(clientIP)
	lease.SubnetMask = net.IPMask(net.ParseIP(subnetMask).To4())
	lease.Router = net.ParseIP(routerIP)
	for i := 0; i < len(DNSServers); i++ {
		lease.DNS = append(lease.DNS, net.ParseIP(DNSServers[i]))
	}
	lease.Reference = ref

	a.leases[hwAddr] = lease

	log.Debugf("(dhcp.AddLease) lease added for hardware address: %s", hwAddr)

	return
}

func (a *DHCPAllocator) CheckLease(hwAddr string) bool {
	_, exists := a.leases[hwAddr]
	return exists
}

func (a *DHCPAllocator) GetLease(hwAddr string) (lease DHCPLease) {
	return a.leases[hwAddr]
}

func (a *DHCPAllocator) DeleteLease(hwAddr string) (err error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if !a.CheckLease(hwAddr) {
		return fmt.Errorf("lease for hwaddr %s does not exists", hwAddr)
	}

	delete(a.leases, hwAddr)

	log.Debugf("(dhcp.DeleteLease) lease deleted for hardware address: %s", hwAddr)

	return
}

func (a *DHCPAllocator) Usage() {
	for hwaddr, lease := range a.leases {
		log.Infof("(dhcp.Usage) lease: hwaddr=%s, clientip=%s, netmask=%s, router=%s, dns=%+v, ref=%s",
			hwaddr,
			lease.ClientIP.String(),
			lease.SubnetMask.String(),
			lease.Router.String(),
			lease.DNS,
			lease.Reference,
		)
	}
}

func New() *DHCPAllocator {
	return NewDHCPAllocator()
}

func (a *DHCPAllocator) dhcpHandler(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4) {
	if m == nil {
		log.Errorf("(dhcp.dhcpHandler) packet is nil!")
		return
	}

	log.Tracef("(dhcp.dhcpHandler) INCOMING PACKET=%s", m.Summary())

	if m.OpCode != dhcpv4.OpcodeBootRequest {
		log.Errorf("(dhcp.dhcpHandler) not a BootRequest!")
		return
	}

	reply, err := dhcpv4.NewReplyFromRequest(m)
	if err != nil {
		log.Errorf("(dhcp.dhcpHandler) NewReplyFromRequest failed: %v", err)
		return
	}

	lease := a.leases[m.ClientHWAddr.String()]

	if lease.ClientIP == nil {
		log.Warnf("(dhcp.dhcpHandler) NO LEASE FOUND: hwaddr=%s", m.ClientHWAddr.String())

		return
	}

	log.Debugf("(dhcp.dhcpHandler) LEASE FOUND: hwaddr=%s, serverip=%s, clientip=%s, mask=%s, router=%s, dns=%+v",
		m.ClientHWAddr.String(),
		lease.ServerIP.String(),
		lease.ClientIP.String(),
		lease.SubnetMask.String(),
		lease.Router.String(),
		lease.DNS,
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
	reply.UpdateOption(dhcpv4.OptDNS(lease.DNS...))

	// TODO: maybe add these options as wel to the IPPool object
	//reply.UpdateOption(dhcpv4.OptBroadcastAddress(net.IP{192, 168, 10, 255}))
	//reply.UpdateOption(dhcpv4.OptClassIdentifier("k8s"))
	//reply.UpdateOption(dhcpv4.OptDomainName("example.com"))

	// TODO: this should be a configuration option in the IPPool object
	// TODO: make this default 1 year instead of 30 minutes (only for testing)
	reply.UpdateOption(dhcpv4.OptIPAddressLeaseTime(30 * time.Minute))

	switch mt := m.MessageType(); mt {
	case dhcpv4.MessageTypeDiscover:
		log.Debugf("(dhcp.dhcpHandler) DHCPDISCOVER: %+v", m)
		reply.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeOffer))
		log.Debugf("(dhcp.dhcpHandler) DHCPOFFER: %+v", reply)
	case dhcpv4.MessageTypeRequest:
		log.Debugf("(dhcp.dhcpHandler) DHCPREQUEST: %+v", m)
		reply.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeAck))
		log.Debugf("(dhcp.dhcpHandler) DHCPACK: %+v", reply)
	default:
		log.Warnf("(dhcp.dhcpHandler) Unhandled message type for hwaddr [%s]: %v", m.ClientHWAddr.String(), mt)
		return
	}

	if _, err := conn.WriteTo(reply.ToBytes(), peer); err != nil {
		log.Errorf("(dhcp.dhcpHandler) Cannot reply to client: %v", err)
	}
}

func (a *DHCPAllocator) Run(nic string, serverip string) (err error) {
	log.Infof("(dhcp.Run) starting DHCP service on nic %s", nic)

	// we need to listen on 0.0.0.0 otherwise client discovers will not be answered
	laddr := net.UDPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: 67,
	}

	server, err := server4.NewServer(nic, &laddr, a.dhcpHandler)
	if err != nil {
		return
	}

	go server.Serve()

	a.servers[nic] = server

	return
}

func (a *DHCPAllocator) Stop(nic string) (err error) {
	log.Infof("(dhcp.Stop) stopping DHCP service on nic %s", nic)

	return a.servers[nic].Close()
}
