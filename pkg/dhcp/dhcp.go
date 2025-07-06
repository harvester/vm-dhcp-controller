package dhcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	// "github.com/harvester/vm-dhcp-controller/pkg/agent" // Avoid direct import for now
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	"github.com/insomniacslk/dhcp/rfc1035label"
)

// DHCPNetConfig is a local representation of network configuration for the DHCP allocator.
// This helps avoid direct import cycles if agent.AgentNetConfig imports dhcp.
type DHCPNetConfig struct {
	InterfaceName string
	ServerIP      string
	CIDR          string
	IPPoolRef     string // Used to associate this config with a specific IPPool
}

type AgentNetConfig struct { // Temporary local definition, assuming it's passed from agent
	InterfaceName string `json:"interfaceName"`
	ServerIP      string `json:"serverIP"`
	CIDR          string `json:"cidr"`
	IPPoolName    string `json:"ipPoolName"`
	IPPoolRef     string `json:"ipPoolRef"`
	NadName       string `json:"nadName"`
	// Misplaced imports removed from here
}

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
	// leases map[string]DHCPLease // Old structure: map[hwAddr]DHCPLease
	leases  map[string]map[string]DHCPLease // New structure: map[ipPoolRef]map[hwAddr]DHCPLease
	servers map[string]*server4.Server      // map[interfaceName]*server4.Server
	mutex   sync.RWMutex
}

func New() *DHCPAllocator {
	return NewDHCPAllocator()
}

func NewDHCPAllocator() *DHCPAllocator {
	// leases := make(map[string]DHCPLease) // Old
	leases := make(map[string]map[string]DHCPLease) // New: map[ipPoolRef]map[hwAddr]DHCPLease
	servers := make(map[string]*server4.Server)     // map[interfaceName]*server4.Server

	return &DHCPAllocator{
		leases:  leases,
		servers: servers,
	}
}

// AddLease now takes an ipPoolRef to store the lease under the correct pool.
func (a *DHCPAllocator) AddLease(
	ipPoolRef string, // New parameter
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

	if ipPoolRef == "" {
		return fmt.Errorf("ipPoolRef is empty")
	}
	if hwAddr == "" {
		return fmt.Errorf("hwaddr is empty")
	}
	if _, err := net.ParseMAC(hwAddr); err != nil {
		return fmt.Errorf("hwaddr %s is not valid", hwAddr)
	}

	if _, ok := a.leases[ipPoolRef]; !ok {
		a.leases[ipPoolRef] = make(map[string]DHCPLease)
	}

	if a.checkLease(ipPoolRef, hwAddr) {
		return fmt.Errorf("lease for hwaddr %s in pool %s already exists", hwAddr, ipPoolRef)
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

	a.leases[ipPoolRef][hwAddr] = lease

	logrus.Infof("(dhcp.AddLease) lease added for hwaddr %s in pool %s", hwAddr, ipPoolRef)
	return
}

// checkLease now takes ipPoolRef.
func (a *DHCPAllocator) checkLease(ipPoolRef string, hwAddr string) bool {
	if poolLeases, ok := a.leases[ipPoolRef]; ok {
		_, exists := poolLeases[hwAddr]
		return exists
	}
	return false
}

// GetLease now takes ipPoolRef.
func (a *DHCPAllocator) GetLease(ipPoolRef string, hwAddr string) (lease DHCPLease, found bool) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	if poolLeases, ok := a.leases[ipPoolRef]; ok {
		lease, found = poolLeases[hwAddr]
		return lease, found
	}
	return DHCPLease{}, false
}

// DeleteLease now takes ipPoolRef.
func (a *DHCPAllocator) DeleteLease(ipPoolRef string, hwAddr string) (err error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if !a.checkLease(ipPoolRef, hwAddr) {
		return fmt.Errorf("lease for hwaddr %s in pool %s does not exist", hwAddr, ipPoolRef)
	}

	delete(a.leases[ipPoolRef], hwAddr)
	if len(a.leases[ipPoolRef]) == 0 {
		delete(a.leases, ipPoolRef)
	}

	logrus.Infof("(dhcp.DeleteLease) lease deleted for hwaddr %s in pool %s", hwAddr, ipPoolRef)
	return
}

func (a *DHCPAllocator) Usage() {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	for ipPoolRef, poolLeases := range a.leases {
		logrus.Infof("(dhcp.Usage) IPPool: %s", ipPoolRef)
		for hwaddr, lease := range poolLeases {
			logrus.Infof("  Lease: hwaddr=%s, clientip=%s, netmask=%s, router=%s, dns=%+v, domain=%s, domainsearch=%+v, ntp=%+v, leasetime=%d",
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
}

// dhcpHandlerPerPool is the actual handler logic, now parameterized by ipPoolRef.
func (a *DHCPAllocator) dhcpHandlerPerPool(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4, ipPoolRef string) {
	// RLock is taken by the caller (the per-interface handler wrapper) or directly if this becomes the main handler again.
	// For now, assume caller handles locking if necessary, or this function does.
	// Let's add lock here for safety, though it might be redundant if wrapped.
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	if m == nil {
		logrus.Errorf("(dhcp.dhcpHandlerPerPool) packet is nil! IPPool: %s", ipPoolRef)
		return
	}

	logrus.Tracef("(dhcp.dhcpHandlerPerPool) INCOMING PACKET=%s on IPPool: %s", m.Summary(), ipPoolRef)

	if m.OpCode != dhcpv4.OpcodeBootRequest {
		logrus.Errorf("(dhcp.dhcpHandlerPerPool) not a BootRequest! IPPool: %s", ipPoolRef)
		return
	}

	reply, err := dhcpv4.NewReplyFromRequest(m)
	if err != nil {
		logrus.Errorf("(dhcp.dhcpHandlerPerPool) NewReplyFromRequest failed for IPPool %s: %v", ipPoolRef, err)
		return
	}

	lease, found := a.GetLease(ipPoolRef, m.ClientHWAddr.String()) // Uses the modified GetLease

	if !found || lease.ClientIP == nil {
		logrus.Warnf("(dhcp.dhcpHandlerPerPool) NO LEASE FOUND: hwaddr=%s for IPPool: %s", m.ClientHWAddr.String(), ipPoolRef)
		return
	}

	logrus.Debugf("(dhcp.dhcpHandlerPerPool) LEASE FOUND for IPPool %s: hwaddr=%s, serverip=%s, clientip=%s, mask=%s, router=%s, dns=%+v, domainname=%s, domainsearch=%+v, ntp=%+v, leasetime=%d",
		ipPoolRef,
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
	reply.ServerIPAddr = lease.ServerIP // This should be the server IP for *this* specific interface/pool
	reply.YourIPAddr = lease.ClientIP
	reply.TransactionID = m.TransactionID
	reply.ClientHWAddr = m.ClientHWAddr
	reply.Flags = m.Flags
	reply.GatewayIPAddr = m.GatewayIPAddr // Usually 0.0.0.0 in client requests, server sets its own if relaying

	reply.UpdateOption(dhcpv4.OptServerIdentifier(lease.ServerIP)) // ServerIP from the lease (specific to this pool)
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
		reply.UpdateOption(dhcpv4.OptIPAddressLeaseTime(31536000 * time.Second)) // Default 1 year
	}

	switch messageType := m.MessageType(); messageType {
	case dhcpv4.MessageTypeDiscover:
		reply.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeOffer))
		logrus.Debugf("(dhcp.dhcpHandlerPerPool) DHCPOFFER for IPPool %s: %+v", ipPoolRef, reply)
	case dhcpv4.MessageTypeRequest:
		reply.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeAck))
		logrus.Debugf("(dhcp.dhcpHandlerPerPool) DHCPACK for IPPool %s: %+v", ipPoolRef, reply)
	default:
		logrus.Warnf("(dhcp.dhcpHandlerPerPool) Unhandled message type for hwaddr [%s] on IPPool %s: %v", m.ClientHWAddr.String(), ipPoolRef, messageType)
		return
	}

	if _, err := conn.WriteTo(reply.ToBytes(), peer); err != nil {
		logrus.Errorf("(dhcp.dhcpHandlerPerPool) Cannot reply to client for IPPool %s: %v", ipPoolRef, err)
	}
}

// Run now accepts a slice of DHCPNetConfig
func (a *DHCPAllocator) Run(ctx context.Context, netConfigs []DHCPNetConfig) (err error) {
	if len(netConfigs) == 0 {
		logrus.Info("(dhcp.Run) no network configurations provided, DHCP server will not start on any interface.")
		return nil
	}

	for _, nc := range netConfigs {
		logrus.Infof("(dhcp.Run) starting DHCP service on nic %s for IPPool %s (ServerIP: %s, CIDR: %s)", nc.InterfaceName, nc.IPPoolRef, nc.ServerIP, nc.CIDR)

		// we need to listen on 0.0.0.0 otherwise client discovers will not be answered
		// The serverIP from nc.ServerIP is used for configuring the interface itself,
		// and for the DHCP ServerIdentifier option. The listener binds to 0.0.0.0 on the specified NIC.
		laddr := net.UDPAddr{
			IP:   net.ParseIP("0.0.0.0"), // Listen on all IPs on the specific interface
			Port: 67,
		}

		// Crucial: Create a new variable for nc.IPPoolRef for the closure
		// to capture the correct value for each iteration.
		currentIPPoolRef := nc.IPPoolRef
		handlerWrapper := func(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4) {
			// Identify the interface the packet came in on. This is tricky with a single 0.0.0.0 listener.
			// server4.NewServer binds to a specific NIC, so conn should be specific to that NIC.
			// The currentIPPoolRef captured by the closure is the key.
			a.dhcpHandlerPerPool(conn, peer, m, currentIPPoolRef)
		}

		server, err := server4.NewServer(nc.InterfaceName, &laddr, handlerWrapper)
		if err != nil {
			logrus.Errorf("(dhcp.Run) failed to create DHCP server on nic %s for IPPool %s: %v", nc.InterfaceName, currentIPPoolRef, err)
			// Decide on error handling: return err, or log and continue with other interfaces?
			// For now, log and continue. Consider returning an aggregated error later.
			continue
		}

		// Use an errgroup for the servers themselves, so if one fails, the context of the group is cancelled.
		// We need a new context for this errgroup, derived from the input ctx.
		// However, the server.Serve() itself should react to the cancellation of the original ctx.
		// The primary purpose of DHCPAllocator.Run is to manage these servers.
		// It should only return when all servers have stopped (due to error or context cancellation).

		// Storing server instances to allow for graceful shutdown via their Close() method.
		a.servers[nc.InterfaceName] = server // Store server instance keyed by interface name

		// This internal goroutine will run server.Serve() and log its lifecycle.
		// It will respect the passed-in ctx for its own termination.
		go func(s *server4.Server, ifName string, poolRef string, Ctx context.Context) {
			logrus.Infof("DHCP server goroutine starting for interface %s (IPPool %s)", ifName, poolRef)
			errServe := s.Serve()
			// Check if context was cancelled, which might cause Serve() to return an error.
			select {
			case <-Ctx.Done():
				// If the context is done, this error is likely related to the context cancellation (e.g., listener closed).
				logrus.Infof("(dhcp.Run) DHCP server on nic %s (IPPool %s) stopped due to context cancellation: %v", ifName, poolRef, errServe)
			default:
				// If context is not done, but Serve() returned, it's an unexpected error.
				logrus.Errorf("(dhcp.Run) DHCP server on nic %s (IPPool %s) exited unexpectedly with error: %v", ifName, poolRef, errServe)
				// This scenario (unexpected error) should ideally trigger a shutdown of the group if using an errgroup here.
				// For now, individual servers exiting due to non-context errors are just logged.
				// A more robust solution would propagate this error to make DHCPAllocator.Run return.
			}
			logrus.Infof("DHCP server goroutine ended for interface %s (IPPool %s)", ifName, poolRef)
		}(server, nc.InterfaceName, currentIPPoolRef, ctx) // Pass the original ctx
	}

	// If no servers were configured or started, return nil.
	if len(a.servers) == 0 {
		logrus.Info("(dhcp.Run) no DHCP servers were actually started.")
		return nil
	}

	// Wait for the context to be cancelled. This will keep DHCPAllocator.Run alive.
	// The actual server goroutines will react to this cancellation.
	// The Cleanup function (called by the agent) will then call stopAll to close servers.
	<-ctx.Done()
	logrus.Info("(dhcp.Run) context cancelled. DHCPAllocator.Run is terminating.")

	// It's important that server.Close() is called to release resources.
	// This is handled by the Cleanup function which calls stopAll().
	// So, DHCPAllocator.Run itself doesn't need to call stopAll() here.
	// It just needs to ensure it stays alive until the context is done.
	return ctx.Err() // Return the context error (e.g., context.Canceled)
}

// DryRun now accepts a slice of DHCPNetConfig
func (a *DHCPAllocator) DryRun(ctx context.Context, netConfigs []DHCPNetConfig) (err error) {
	if len(netConfigs) == 0 {
		logrus.Info("(dhcp.DryRun) no network configurations provided.")
		return nil
	}
	for _, nc := range netConfigs {
		logrus.Infof("(dhcp.DryRun) simulating DHCP service start on nic %s for IPPool %s (ServerIP: %s, CIDR: %s)",
			nc.InterfaceName, nc.IPPoolRef, nc.ServerIP, nc.CIDR)
		// In a real dry run, you might do more checks, but for now, just log.
		// No actual server is started or stored in a.servers for dry run.
	}
	return nil
}

// stopAll stops all running DHCP servers.
func (a *DHCPAllocator) stopAll() error {
	a.mutex.Lock() // Lock to safely access a.servers
	defer a.mutex.Unlock()

	logrus.Infof("(dhcp.stopAll) stopping all DHCP services...")
	var Merror error
	for ifName, server := range a.servers {
		if server != nil {
			logrus.Infof("(dhcp.stopAll) stopping DHCP service on nic %s", ifName)
			if err := server.Close(); err != nil {
				logrus.Errorf("(dhcp.stopAll) error stopping server on nic %s: %v", ifName, err)
				if Merror == nil {
					Merror = fmt.Errorf("error stopping server on nic %s: %w", ifName, err)
				} else {
					Merror = fmt.Errorf("%v; error stopping server on nic %s: %w", Merror, ifName, err)
				}
			}
			delete(a.servers, ifName) // Remove from map after stopping
		}
	}
	if Merror != nil {
		logrus.Errorf("(dhcp.stopAll) finished stopping services with errors: %v", Merror)
		return Merror
	}
	logrus.Info("(dhcp.stopAll) successfully stopped all DHCP services.")
	return nil
}

// ListAll now needs to consider the new lease structure.
// The 'name' parameter seems unused; let's clarify its purpose or remove it.
// For now, it will list all leases from all pools.
func (a *DHCPAllocator) ListAll(_ string) (map[string]string, error) { // name param is unused
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	allLeasesFlat := make(map[string]string)
	for ipPoolRef, poolLeases := range a.leases {
		for mac, lease := range poolLeases {
			leaseKey := fmt.Sprintf("%s/%s", ipPoolRef, mac) // e.g., "default/myippool/00:11:22:33:44:55"
			allLeasesFlat[leaseKey] = lease.String()
		}
	}
	return allLeasesFlat, nil
}

// Cleanup now calls stopAll. The 'nic' parameter is no longer needed.
func Cleanup(ctx context.Context, a *DHCPAllocator) <-chan error {
	errCh := make(chan error, 1) // Buffered channel

	go func() {
		<-ctx.Done()
		logrus.Info("(dhcp.Cleanup) context done, stopping all DHCP servers.")
		errCh <- a.stopAll()
		close(errCh)
	}()

	return errCh
}
