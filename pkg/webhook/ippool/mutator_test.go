package ippool

import (
	"fmt"
	"testing"

	networkv1 "github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/harvester/webhook/pkg/server/admission"
	"github.com/stretchr/testify/assert"
)

func TestMutator_Create(t *testing.T) {
	type input struct {
		name   string
		ipPool *networkv1.IPPool
	}
	type output struct {
		patch admission.Patch
		err   error
	}
	testCases := []struct {
		given    input
		expected output
	}{
		{
			given: input{
				name: "no router ippool with server, start, and end ips undefined",
				ipPool: newTestIPPoolBuilder().
					CIDR("192.168.0.0/24").Build(),
			},
			expected: output{
				patch: admission.Patch{
					{
						Op:   admission.PatchOpReplace,
						Path: "/spec/ipv4Config/pool",
						Value: networkv1.Pool{
							Start: "192.168.0.1",
							End:   "192.168.0.254",
						},
					},
					{
						Op:    admission.PatchOpReplace,
						Path:  "/spec/ipv4Config/serverIP",
						Value: "192.168.0.1",
					},
				},
			},
		},
		{
			given: input{
				name: "ippool with server, start, and end ips undefined",
				ipPool: newTestIPPoolBuilder().
					CIDR("172.19.64.128/29").
					Router("172.19.64.129").Build(),
			},
			expected: output{
				patch: admission.Patch{
					{
						Op:   admission.PatchOpReplace,
						Path: "/spec/ipv4Config/pool",
						Value: networkv1.Pool{
							Start: "172.19.64.129",
							End:   "172.19.64.134",
						},
					},
					{
						Op:    admission.PatchOpReplace,
						Path:  "/spec/ipv4Config/serverIP",
						Value: "172.19.64.130",
					},
				},
			},
		},
		{
			given: input{
				name: "ippool with server, start, and end ips all defined",
				ipPool: newTestIPPoolBuilder().
					CIDR("172.19.64.128/29").
					ServerIP("172.19.64.130").
					Router("172.19.64.129").
					PoolRange("172.19.64.131", "172.19.64.133").Build(),
			},
			expected: output{},
		},
		{
			given: input{
				name: "/30 ippool with router (zero allocatable ip left effectively)",
				ipPool: newTestIPPoolBuilder().
					CIDR("172.19.64.128/30").
					Router("172.19.64.129").Build(),
			},
			expected: output{
				patch: admission.Patch{
					{
						Op:   admission.PatchOpReplace,
						Path: "/spec/ipv4Config/pool",
						Value: networkv1.Pool{
							Start: "172.19.64.129",
							End:   "172.19.64.130",
						},
					},
					{
						Op:    admission.PatchOpReplace,
						Path:  "/spec/ipv4Config/serverIP",
						Value: "172.19.64.130",
					},
				},
			},
		},
		{
			given: input{
				name: "/30 ippool without router (one allocatable ip left effectively)",
				ipPool: newTestIPPoolBuilder().
					CIDR("172.19.64.128/30").Build(),
			},
			expected: output{
				patch: admission.Patch{
					{
						Op:   admission.PatchOpReplace,
						Path: "/spec/ipv4Config/pool",
						Value: networkv1.Pool{
							Start: "172.19.64.129",
							End:   "172.19.64.130",
						},
					},
					{
						Op:    admission.PatchOpReplace,
						Path:  "/spec/ipv4Config/serverIP",
						Value: "172.19.64.129",
					},
				},
			},
		},
		{
			given: input{
				name: "/30 ippool without router but with start ip defined",
				ipPool: newTestIPPoolBuilder().
					CIDR("172.19.64.128/30").
					PoolRange("172.19.64.130", "").Build(),
			},
			expected: output{
				patch: admission.Patch{
					{
						Op:   admission.PatchOpReplace,
						Path: "/spec/ipv4Config/pool",
						Value: networkv1.Pool{
							Start: "172.19.64.130",
							End:   "172.19.64.130",
						},
					},
					{
						Op:    admission.PatchOpReplace,
						Path:  "/spec/ipv4Config/serverIP",
						Value: "172.19.64.129",
					},
				},
			},
		},
		{
			given: input{
				name: "no router ippool with excluded ips",
				ipPool: newTestIPPoolBuilder().
					CIDR("172.19.64.128/29").
					Exclude("172.19.64.129", "172.19.64.131").Build(),
			},
			expected: output{
				patch: admission.Patch{
					{
						Op:   admission.PatchOpReplace,
						Path: "/spec/ipv4Config/pool",
						Value: networkv1.Pool{
							Start: "172.19.64.129",
							End:   "172.19.64.134",
							Exclude: []string{
								"172.19.64.129",
								"172.19.64.131",
							},
						},
					},
					{
						Op:    admission.PatchOpReplace,
						Path:  "/spec/ipv4Config/serverIP",
						Value: "172.19.64.130",
					},
				},
			},
		},
		{
			given: input{
				name: "the only available ip left is the broadcast ip",
				ipPool: newTestIPPoolBuilder().
					CIDR("172.19.64.128/29").
					Router("172.19.64.130").
					Exclude("172.19.64.129", "172.19.64.131", "172.19.64.132", "172.19.64.133", "172.19.64.134").Build(),
			},
			expected: output{
				err: fmt.Errorf("cannot create IPPool %s/%s because fail to assign ip for dhcp server", testIPPoolNamespace, testIPPoolName),
			},
		},
		{
			given: input{
				name: "/32 ippool",
				ipPool: newTestIPPoolBuilder().
					CIDR("172.19.64.128/32").Build(),
			},
			expected: output{
				err: fmt.Errorf("cannot create IPPool %s/%s because fail to assign ip for dhcp server", testIPPoolNamespace, testIPPoolName),
			},
		},
		{
			given: input{
				name: "/31 ippool",
				ipPool: newTestIPPoolBuilder().
					CIDR("172.19.64.128/31").Build(),
			},
			expected: output{
				err: fmt.Errorf("cannot create IPPool %s/%s because fail to assign ip for dhcp server", testIPPoolNamespace, testIPPoolName),
			},
		},
		{
			given: input{
				name: "server ip and router are in the middle of pool range",
				ipPool: newTestIPPoolBuilder().
					CIDR("192.168.0.0/24").
					ServerIP("192.168.0.50").
					Router("192.168.0.100").Build(),
			},
			expected: output{
				patch: admission.Patch{
					{
						Op:   admission.PatchOpReplace,
						Path: "/spec/ipv4Config/pool",
						Value: networkv1.Pool{
							Start: "192.168.0.1",
							End:   "192.168.0.254",
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		mutator := NewMutator()

		patch, err := mutator.Create(&admission.Request{}, tc.given.ipPool)
		if tc.expected.err != nil {
			assert.Equal(t, tc.expected.err.Error(), err.Error(), tc.given.name)
		} else {
			assert.Nil(t, err, tc.given.name)
		}
		assert.Equal(t, tc.expected.patch, patch, tc.given.name)
	}
}
