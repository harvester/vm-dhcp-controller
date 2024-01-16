package dhcp

import (
	"fmt"
	"net"
	"testing"
)

func TestDHCP(t *testing.T) {
	td := New()

	testLeases := []struct {
		hwAddr       string
		serverIP     string
		clientIP     string
		cidr         string
		routerIP     string
		dnsServers   []string
		domainName   *string
		domainSearch []string
		ntpServers   []string
		leaseTime    *int
		want         error
	}{
		{
			hwAddr:       "aa:bb:cc:dd:ee:ff",
			serverIP:     "0.0.0.0",
			clientIP:     "192.168.0.10",
			cidr:         "192.168.0.0/24",
			routerIP:     "192.168.0.254",
			dnsServers:   []string{"8.8.8.8", "8.8.4.4"},
			domainName:   func(s string) *string { return &s }("example.com"),
			domainSearch: []string{"example.com"},
			ntpServers:   []string{"localhost", "127.0.0.2"},
			leaseTime:    func(i int) *int { return &i }(300),
			want:         nil,
		},
		{
			hwAddr:       "aa:bb:cc:dd:ee:ff",
			serverIP:     "0.0.0.0",
			clientIP:     "192.168.0.10",
			cidr:         "192.168.0.0/24",
			routerIP:     "192.168.0.254",
			dnsServers:   []string{"8.8.8.8", "8.8.4.4"},
			domainName:   func(s string) *string { return &s }("example.com"),
			domainSearch: []string{"example.com"},
			ntpServers:   []string{"localhost", "127.0.0.2"},
			leaseTime:    func(i int) *int { return &i }(300),
			want:         fmt.Errorf("lease for hwaddr aa:bb:cc:dd:ee:ff already exists"),
		},
		{
			hwAddr:       "00:01:02:03:04:05",
			serverIP:     "0.0.0.0",
			clientIP:     "192.168.0.11",
			cidr:         "192.168.0.0/24",
			routerIP:     "192.168.0.254",
			dnsServers:   []string{"8.8.8.8", "8.8.4.4"},
			domainName:   func(s string) *string { return &s }("example.com"),
			domainSearch: []string{"example.com"},
			ntpServers:   []string{"localhost", "127.0.0.2"},
			leaseTime:    func(i int) *int { return &i }(300),
			want:         nil,
		},
		{
			hwAddr:       "01:02:03:04:05:06",
			serverIP:     "0.0.0.0",
			clientIP:     "",
			cidr:         "192.168.0.0/24",
			routerIP:     "192.168.0.254",
			dnsServers:   []string{"8.8.8.8", "8.8.4.4"},
			domainName:   func(s string) *string { return &s }("example.com"),
			domainSearch: []string{"example.com"},
			ntpServers:   []string{"localhost", "127.0.0.2"},
			leaseTime:    func(i int) *int { return &i }(300),
			want:         nil,
		},
		{
			hwAddr:       "",
			serverIP:     "0.0.0.0",
			clientIP:     "",
			cidr:         "192.168.0.0/24",
			routerIP:     "192.168.0.254",
			dnsServers:   []string{"8.8.8.8", "8.8.4.4"},
			domainName:   func(s string) *string { return &s }("example.com"),
			domainSearch: []string{"example.com"},
			ntpServers:   []string{"localhost", "127.0.0.2"},
			leaseTime:    func(i int) *int { return &i }(300),
			want:         fmt.Errorf("hwaddr is empty"),
		},
		{
			hwAddr:   "11:22:33:44:55",
			serverIP: "0.0.0.0",
			clientIP: "",
			cidr:     "192.168.0.0/24",
			routerIP: "",
			want:     fmt.Errorf("hwaddr 11:22:33:44:55 is not valid"),
		},
		{
			hwAddr:   "11:22:33:44:55",
			serverIP: "0.0.0.0",
			clientIP: "",
			cidr:     "192.168.0.0/24",
			routerIP: "192.168.0.254",
			want:     fmt.Errorf("hwaddr 11:22:33:44:55 is not valid"),
		},
		{
			hwAddr:   "11:22:33:44:55:66",
			serverIP: "0.0.0.0",
			clientIP: "",
			cidr:     "192.168.0.0/36",
			routerIP: "192.168.0.254",
			want:     fmt.Errorf("invalid CIDR address: 192.168.0.0/36"),
		},
		{
			hwAddr:     "11:22:33:44:55:66",
			serverIP:   "0.0.0.0",
			clientIP:   "",
			cidr:       "192.168.0.0/24",
			routerIP:   "192.168.0.254",
			ntpServers: []string{"xxxx"},
			want:       nil,
		},
	}

	// AddLease function tests
	for i := 0; i < len(testLeases); i++ {
		if got := td.AddLease(
			testLeases[i].hwAddr,
			testLeases[i].serverIP,
			testLeases[i].clientIP,
			testLeases[i].cidr,
			testLeases[i].routerIP,
			testLeases[i].dnsServers,
			testLeases[i].domainName,
			testLeases[i].domainSearch,
			testLeases[i].ntpServers,
			testLeases[i].leaseTime,
		); got != testLeases[i].want {
			if got == nil || testLeases[i].want == nil {
				t.Errorf("got %q, wanted %q", got, testLeases[i].want)
			} else if got.Error() != testLeases[i].want.Error() {
				t.Errorf("got %q, wanted %q", got, testLeases[i].want)
			}
		}
	}

	// GetLease function tests
	lease1 := td.GetLease("aa:bb:cc:dd:ee:ff")
	if !lease1.ClientIP.Equal(net.ParseIP(testLeases[0].clientIP)) {
		t.Errorf("got %q, wanted %q", lease1.ClientIP.String(), testLeases[1].clientIP)
	}
	lease2 := td.GetLease("ff:ee:dd:cc:bb:aa")
	if len(lease2.ClientIP) > 0 {
		t.Errorf("got %q, wanted nil", lease2.ClientIP.String())
	}

	// checkLease function tests
	if !td.checkLease("aa:bb:cc:dd:ee:ff") {
		t.Errorf("got false, wanted true for hwAddr aa:bb:cc:dd:ee:ff")
	}
	if td.checkLease("00:11:22:33:44:55") {
		t.Errorf("got true, wanted false for hwAddr 00:11:22:33:44:55")
	}
	if td.checkLease("ff:ee:dd:cc:bb:aa") {
		t.Errorf("got true, wanted false for hwAddr ff:ee:dd:cc:bb:aa")
	}
	if !td.checkLease("00:01:02:03:04:05") {
		t.Errorf("got false, wanted true for hwAddr 00:01:02:03:04:05")
	}

	// DeleteLease function tests
	if got := td.DeleteLease("aa:bb:cc:dd:ee:ff"); got != nil {
		t.Errorf("got %q, wanted nil", got)
	}
	if got := td.DeleteLease("aa:bb:cc:dd:ee:ff"); got != nil {
		wanted := "lease for hwaddr aa:bb:cc:dd:ee:ff does not exists"
		if got.Error() != wanted {
			t.Errorf("got %q, wanted %q", got, wanted)
		}
	}
}
