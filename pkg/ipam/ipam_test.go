package ipam

import (
	"fmt"
	"testing"
)

func TestIPAM(t *testing.T) {
	ti := New()

	testIPSubnets := []struct {
		name  string
		cidr  string
		start string
		end   string
		want  error
	}{
		{
			name:  "default/network-class-c-ok",
			cidr:  "192.168.0.0/24",
			start: "192.168.0.10",
			end:   "192.168.0.254",
			want:  nil,
		},
		{
			name:  "default/network-class-c-start-error",
			cidr:  "192.168.0.0/24",
			start: "192.168.1.10",
			end:   "192.168.0.254",
			want:  fmt.Errorf("start ip address 192.168.1.10 is not within subnet 192.168.0.0/24 range"),
		},
		{
			name:  "default/network-class-c-end-error",
			cidr:  "192.168.0.0/24",
			start: "192.168.0.10",
			end:   "192.168.1.125",
			want:  fmt.Errorf("end ip address 192.168.1.125 is not within subnet 192.168.0.0/24 range"),
		},
		{
			name:  "default/network-class-c-smaller-error",
			cidr:  "192.168.0.0/24",
			start: "192.168.0.200",
			end:   "192.168.0.100",
			want:  fmt.Errorf("end ip address 192.168.0.100 is less than start ip address 192.168.0.200"),
		},
		{
			name:  "default/network-class-c-broadcast-error",
			cidr:  "192.168.0.0/24",
			start: "192.168.0.10",
			end:   "192.168.0.255",
			want:  fmt.Errorf("end ip address 192.168.0.255 equals broadcast ip address 192.168.0.255"),
		},
		{
			name:  "default/network-class-b-ok",
			cidr:  "172.16.0.0/16",
			start: "172.16.0.10",
			end:   "172.16.255.254",
			want:  nil,
		},
		{
			name:  "default/network-class-b-start-error",
			cidr:  "172.16.0.0/16",
			start: "172.10.0.10",
			end:   "172.16.254.254",
			want:  fmt.Errorf("start ip address 172.10.0.10 is not within subnet 172.16.0.0/16 range"),
		},
		{
			name:  "default/network-class-b-end-error",
			cidr:  "172.16.0.0/16",
			start: "172.16.0.10",
			end:   "172.200.1.125",
			want:  fmt.Errorf("end ip address 172.200.1.125 is not within subnet 172.16.0.0/16 range"),
		},
		{
			name:  "default/network-class-b-smaller-error",
			cidr:  "172.16.0.0/16",
			start: "172.16.180.10",
			end:   "172.16.0.100",
			want:  fmt.Errorf("end ip address 172.16.0.100 is less than start ip address 172.16.180.10"),
		},
		{
			name:  "default/network-class-b-broadcast-error",
			cidr:  "172.16.0.0/16",
			start: "172.16.0.10",
			end:   "172.16.255.255",
			want:  fmt.Errorf("end ip address 172.16.255.255 equals broadcast ip address 172.16.255.255"),
		},
		{
			name:  "default/network-class-a-ok",
			cidr:  "10.0.0.0/8",
			start: "10.0.0.10",
			end:   "10.255.255.254",
			want:  nil,
		},
		{
			name:  "default/network-class-a-start-error",
			cidr:  "10.0.0.0/8",
			start: "11.0.0.10",
			end:   "10.255.255.254",
			want:  fmt.Errorf("start ip address 11.0.0.10 is not within subnet 10.0.0.0/8 range"),
		},
		{
			name:  "default/network-class-a-end-error",
			cidr:  "10.0.0.0/8",
			start: "10.0.0.10",
			end:   "250.255.255.254",
			want:  fmt.Errorf("end ip address 250.255.255.254 is not within subnet 10.0.0.0/8 range"),
		},
		{
			name:  "default/network-class-a-smaller-error",
			cidr:  "10.0.0.0/8",
			start: "10.255.255.253",
			end:   "10.10.227.10",
			want:  fmt.Errorf("end ip address 10.10.227.10 is less than start ip address 10.255.255.253"),
		},
		{
			name:  "default/network-class-a-broadcast-error",
			cidr:  "10.0.0.0/8",
			start: "10.0.0.10",
			end:   "10.255.255.255",
			want:  fmt.Errorf("end ip address 10.255.255.255 equals broadcast ip address 10.255.255.255"),
		},
		{
			name:  "default/network-class-c-small",
			cidr:  "192.168.10.64/31",
			start: "192.168.10.64",
			end:   "192.168.10.64",
			want:  nil,
		},
	}

	// NewIPSubnet function tests
	for i := 0; i < len(testIPSubnets); i++ {
		if got := ti.NewIPSubnet(
			testIPSubnets[i].name,
			testIPSubnets[i].cidr,
			testIPSubnets[i].start,
			testIPSubnets[i].end,
		); got != testIPSubnets[i].want {
			if got == nil || testIPSubnets[i].want == nil {
				t.Errorf("got %q, wanted %q", got, testIPSubnets[i].want)
			} else if got.Error() != testIPSubnets[i].want.Error() {
				t.Errorf("got %q, wanted %q", got, testIPSubnets[i].want)
			}
		}
	}

	allocateIPs := []struct {
		subnetName string
		ip         string
		want       error
	}{
		{
			subnetName: "default/not-existing-network-class",
			ip:         "",
			want:       fmt.Errorf("network default/not-existing-network-class does not exist"),
		},
		{
			subnetName: "default/network-class-c-ok",
			ip:         "192.168.0.58",
			want:       nil,
		},
		{
			subnetName: "default/network-class-c-ok",
			ip:         "192.168.1.190",
			want:       fmt.Errorf("designated ip 192.168.1.190 is not in subnet 192.168.0.0/24"),
		},
		{
			subnetName: "default/network-class-b-ok",
			ip:         "172.16.0.11",
			want:       nil,
		},
		{
			subnetName: "default/network-class-b-ok",
			ip:         "172.16.255.255",
			want:       fmt.Errorf("designated ip 172.16.255.255 equals broadcast ip address 172.16.255.255"),
		},
		{
			subnetName: "default/network-class-b-ok",
			ip:         "172.16.0.11",
			want:       fmt.Errorf("designated ip 172.16.0.11 is already allocated"),
		},
		{
			subnetName: "default/network-class-b-ok",
			ip:         "172.16.0.10",
			want:       nil,
		},
		{
			subnetName: "default/network-class-b-ok",
			ip:         "",
			want:       nil,
		},
		{
			subnetName: "default/network-class-c-small",
			ip:         "",
			want:       nil,
		},
		{
			subnetName: "default/network-class-c-small",
			ip:         "",
			want:       fmt.Errorf("no more ip addresses left in network default/network-class-c-small ipam"),
		},
	}

	// AllocateIP function tests
	for i := 0; i < len(allocateIPs); i++ {
		_, got := ti.AllocateIP(
			allocateIPs[i].subnetName,
			allocateIPs[i].ip,
		)
		if got != allocateIPs[i].want {
			if got == nil || allocateIPs[i].want == nil {
				t.Errorf("got %q, wanted %q", got, allocateIPs[i].want)
			} else if got.Error() != allocateIPs[i].want.Error() {
				t.Errorf("got %q, wanted %q", got, allocateIPs[i].want)
			}
		}
	}

	deallocateIPs := []struct {
		subnetName string
		ip         string
		want       error
	}{
		{
			subnetName: "default/not-existing-network-class",
			ip:         "",
			want:       fmt.Errorf("network default/not-existing-network-class does not exist"),
		},
		{
			subnetName: "default/network-class-b-ok",
			ip:         "172.16.0.11",
			want:       nil,
		},
		{
			subnetName: "default/network-class-b-ok",
			ip:         "",
			want:       fmt.Errorf("designated ip is empty"),
		},
		{
			subnetName: "default/network-class-b-ok",
			ip:         "172.18.128.129",
			want:       fmt.Errorf("to-be-deallocated ip 172.18.128.129 was not found in network default/network-class-b-ok ipam"),
		},
		{
			subnetName: "default/network-class-b-ok",
			ip:         "172.16.0.11",
			want:       fmt.Errorf("to-be-deallocated ip 172.16.0.11 was not allocated"),
		},
		{
			subnetName: "default/network-class-b-ok",
			ip:         "172.16.0.5",
			want:       fmt.Errorf("to-be-deallocated ip 172.16.0.5 was not found in network default/network-class-b-ok ipam"),
		},
	}

	// DeallocateIP function tests
	for i := 0; i < len(deallocateIPs); i++ {
		if got := ti.DeallocateIP(
			deallocateIPs[i].subnetName,
			deallocateIPs[i].ip,
		); got != deallocateIPs[i].want {
			if got == nil || deallocateIPs[i].want == nil {
				t.Errorf("got %q, wanted %q", got, deallocateIPs[i].want)
			} else if got.Error() != deallocateIPs[i].want.Error() {
				t.Errorf("got %q, wanted %q", got, deallocateIPs[i].want)
			}
		}
	}

	// GetUsed and GetAvailable funtion tests
	used, err := ti.GetUsed("default/network-class-b-ok")
	if err != nil {
		t.Errorf("%s", err.Error())
	}
	if used != 2 {
		t.Errorf("got %d, wanted 2", used)
	}
	available, err := ti.GetAvailable("default/network-class-b-ok")
	if err != nil {
		t.Errorf("%s", err.Error())
	}
	if available != 65523 {
		t.Errorf("got %d, wanted 65523", available)
	}

	// DeleteIPSubnet funtion tests
	ti.DeleteIPSubnet("default/network-class-c-ok")
	_, got := ti.AllocateIP("default/network-class-c-ok", "")
	if got == nil {
		t.Errorf("network default/network-class-c-ok still exists")
	} else if got.Error() != "network default/network-class-c-ok does not exist" {
		t.Errorf("got %q", got)
	}
}
