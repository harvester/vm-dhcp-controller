package vmnetcfg

import (
	"testing"

	"github.com/harvester/webhook/pkg/server/admission"
	cniv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/stretchr/testify/assert"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	networkv1 "github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/harvester/vm-dhcp-controller/pkg/controller/ippool"
	"github.com/harvester/vm-dhcp-controller/pkg/controller/vmnetcfg"
	"github.com/harvester/vm-dhcp-controller/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/vm-dhcp-controller/pkg/util"
	"github.com/harvester/vm-dhcp-controller/pkg/util/fakeclient"
)

const (
	testVmNetCfgNamespace = "default"
	testVmNetCfgName      = "test-vmnetcfg"
	testIPPoolNamespace   = "default"
	testIPPoolName        = "test-pool"
	testNADNamespace      = "default"
	testNADName           = "test-net"
	testNetworkName       = testNADNamespace + "/" + testNADName
	testCIDR              = "192.168.0.0/24"
	testStartIP           = "192.168.0.100"
	testEndIP             = "192.168.0.200"
	testIPAddress1        = "192.168.0.111"
	testIPAddress2        = "192.168.0.112"
	testOutsideCIDRIP     = "192.168.1.111"
	testOutsideRangeIP    = "192.168.0.50"
	testMACAddress1       = "11:22:33:44:55:66"
	testMACAddress2       = "22:33:44:55:66:77"
)

func newTestVirtualMachineNetworkConfigBuilder() *vmnetcfg.VmNetCfgBuilder {
	return vmnetcfg.NewVmNetCfgBuilder(testVmNetCfgNamespace, testVmNetCfgName)
}

func newTestIPPoolBuilder() *ippool.IPPoolBuilder {
	return ippool.NewIPPoolBuilder(testIPPoolNamespace, testIPPoolName)
}

func newTestNetworkAttachmentDefinitionBuilder() *ippool.NetworkAttachmentDefinitionBuilder {
	return ippool.NewNetworkAttachmentDefinitionBuilder(testNADNamespace, testNADName)
}

func TestValidator_Create(t *testing.T) {
	type input struct {
		vmNetCfg *networkv1.VirtualMachineNetworkConfig
		ipPool   *networkv1.IPPool
		nad      *cniv1.NetworkAttachmentDefinition
	}

	type output struct {
		shouldErr bool
	}

	testCases := []struct {
		name     string
		given    input
		expected output
	}{
		{
			name: "ippool name different from nad name with the nad properly labeled",
			given: input{
				vmNetCfg: newTestVirtualMachineNetworkConfigBuilder().
					WithNetworkConfig("", "", testNetworkName).Build(),
				ipPool: newTestIPPoolBuilder().
					NetworkName(testNetworkName).Build(),
				nad: newTestNetworkAttachmentDefinitionBuilder().
					Label(util.IPPoolNamespaceLabelKey, testIPPoolNamespace).
					Label(util.IPPoolNameLabelKey, testIPPoolName).Build(),
			},
			expected: output{
				shouldErr: false,
			},
		},
		{
			name: "ippool and nad share the same name with the nad properly labeled",
			given: input{
				vmNetCfg: newTestVirtualMachineNetworkConfigBuilder().
					WithNetworkConfig("", "", testNetworkName).Build(),
				ipPool: ippool.NewIPPoolBuilder(testNADNamespace, testNADName).
					NetworkName(testNetworkName).Build(),
				nad: newTestNetworkAttachmentDefinitionBuilder().
					Label(util.IPPoolNamespaceLabelKey, testNADNamespace).
					Label(util.IPPoolNameLabelKey, testNADName).Build(),
			},
			expected: output{
				shouldErr: false,
			},
		},
		{
			name: "non-existed ippool referenced",
			given: input{
				vmNetCfg: newTestVirtualMachineNetworkConfigBuilder().
					WithNetworkConfig("", "", testNetworkName).Build(),
				nad: newTestNetworkAttachmentDefinitionBuilder().
					Label(util.IPPoolNamespaceLabelKey, testIPPoolNamespace).
					Label(util.IPPoolNameLabelKey, testIPPoolName).Build(),
			},
			expected: output{
				shouldErr: true,
			},
		},
		{
			name: "non-existed nad referenced",
			given: input{
				vmNetCfg: newTestVirtualMachineNetworkConfigBuilder().
					WithNetworkConfig("", "", testNetworkName).Build(),
				ipPool: newTestIPPoolBuilder().
					NetworkName(testNetworkName).Build(),
			},
			expected: output{
				shouldErr: true,
			},
		},
		{
			name: "nad referenced does not have proper labels to associate with the target ippool",
			given: input{
				vmNetCfg: newTestVirtualMachineNetworkConfigBuilder().
					WithNetworkConfig("", "", testNetworkName).Build(),
				ipPool: newTestIPPoolBuilder().Build(),
				nad:    newTestNetworkAttachmentDefinitionBuilder().Build(),
			},
			expected: output{
				shouldErr: true,
			},
		},
		{
			name: "nad referenced has incomplete labels set to associate with the target ippool",
			given: input{
				vmNetCfg: newTestVirtualMachineNetworkConfigBuilder().
					WithNetworkConfig("", "", testNetworkName).Build(),
				ipPool: newTestIPPoolBuilder().Build(),
				nad: newTestNetworkAttachmentDefinitionBuilder().
					Label(util.IPPoolNameLabelKey, testIPPoolName).Build(),
			},
			expected: output{
				shouldErr: true,
			},
		},
		{
			name: "desired static ip inside pool is allowed",
			given: input{
				vmNetCfg: newTestVirtualMachineNetworkConfigBuilder().
					WithNetworkConfig(testIPAddress1, testMACAddress1, testNetworkName).Build(),
				ipPool: newTestIPPoolBuilder().
					NetworkName(testNetworkName).
					CIDR(testCIDR).
					PoolRange(testStartIP, testEndIP).Build(),
				nad: newTestNetworkAttachmentDefinitionBuilder().
					Label(util.IPPoolNamespaceLabelKey, testIPPoolNamespace).
					Label(util.IPPoolNameLabelKey, testIPPoolName).Build(),
			},
			expected: output{
				shouldErr: false,
			},
		},
		{
			name: "desired static ip outside cidr is denied",
			given: input{
				vmNetCfg: newTestVirtualMachineNetworkConfigBuilder().
					WithNetworkConfig(testOutsideCIDRIP, testMACAddress1, testNetworkName).Build(),
				ipPool: newTestIPPoolBuilder().
					NetworkName(testNetworkName).
					CIDR(testCIDR).
					PoolRange(testStartIP, testEndIP).Build(),
				nad: newTestNetworkAttachmentDefinitionBuilder().
					Label(util.IPPoolNamespaceLabelKey, testIPPoolNamespace).
					Label(util.IPPoolNameLabelKey, testIPPoolName).Build(),
			},
			expected: output{
				shouldErr: true,
			},
		},
		{
			name: "desired static ip outside pool range is denied",
			given: input{
				vmNetCfg: newTestVirtualMachineNetworkConfigBuilder().
					WithNetworkConfig(testOutsideRangeIP, testMACAddress1, testNetworkName).Build(),
				ipPool: newTestIPPoolBuilder().
					NetworkName(testNetworkName).
					CIDR(testCIDR).
					PoolRange(testStartIP, testEndIP).Build(),
				nad: newTestNetworkAttachmentDefinitionBuilder().
					Label(util.IPPoolNamespaceLabelKey, testIPPoolNamespace).
					Label(util.IPPoolNameLabelKey, testIPPoolName).Build(),
			},
			expected: output{
				shouldErr: true,
			},
		},
		{
			name: "desired static ip marked excluded is denied",
			given: input{
				vmNetCfg: newTestVirtualMachineNetworkConfigBuilder().
					WithNetworkConfig(testIPAddress1, testMACAddress1, testNetworkName).Build(),
				ipPool: newTestIPPoolBuilder().
					NetworkName(testNetworkName).
					CIDR(testCIDR).
					PoolRange(testStartIP, testEndIP).
					Allocated(testIPAddress1, util.ExcludedMark).Build(),
				nad: newTestNetworkAttachmentDefinitionBuilder().
					Label(util.IPPoolNamespaceLabelKey, testIPPoolNamespace).
					Label(util.IPPoolNameLabelKey, testIPPoolName).Build(),
			},
			expected: output{
				shouldErr: true,
			},
		},
		{
			name: "desired static ip marked reserved is denied",
			given: input{
				vmNetCfg: newTestVirtualMachineNetworkConfigBuilder().
					WithNetworkConfig(testIPAddress1, testMACAddress1, testNetworkName).Build(),
				ipPool: newTestIPPoolBuilder().
					NetworkName(testNetworkName).
					CIDR(testCIDR).
					PoolRange(testStartIP, testEndIP).
					Allocated(testIPAddress1, util.ReservedMark).Build(),
				nad: newTestNetworkAttachmentDefinitionBuilder().
					Label(util.IPPoolNamespaceLabelKey, testIPPoolNamespace).
					Label(util.IPPoolNameLabelKey, testIPPoolName).Build(),
			},
			expected: output{
				shouldErr: true,
			},
		},
		{
			name: "desired static ip allocated to a different mac is denied",
			given: input{
				vmNetCfg: newTestVirtualMachineNetworkConfigBuilder().
					WithNetworkConfig(testIPAddress1, testMACAddress1, testNetworkName).Build(),
				ipPool: newTestIPPoolBuilder().
					NetworkName(testNetworkName).
					CIDR(testCIDR).
					PoolRange(testStartIP, testEndIP).
					Allocated(testIPAddress1, testMACAddress2).Build(),
				nad: newTestNetworkAttachmentDefinitionBuilder().
					Label(util.IPPoolNamespaceLabelKey, testIPPoolNamespace).
					Label(util.IPPoolNameLabelKey, testIPPoolName).Build(),
			},
			expected: output{
				shouldErr: true,
			},
		},
		{
			name: "duplicate desired static ip on same network is denied",
			given: input{
				vmNetCfg: newTestVirtualMachineNetworkConfigBuilder().
					WithNetworkConfig(testIPAddress1, testMACAddress1, testNetworkName).
					WithNetworkConfig(testIPAddress1, testMACAddress2, testNetworkName).Build(),
				ipPool: newTestIPPoolBuilder().
					NetworkName(testNetworkName).
					CIDR(testCIDR).
					PoolRange(testStartIP, testEndIP).Build(),
				nad: newTestNetworkAttachmentDefinitionBuilder().
					Label(util.IPPoolNamespaceLabelKey, testIPPoolNamespace).
					Label(util.IPPoolNameLabelKey, testIPPoolName).Build(),
			},
			expected: output{
				shouldErr: true,
			},
		},
	}

	nadGVR := schema.GroupVersionResource{
		Group:    "k8s.cni.cncf.io",
		Version:  "v1",
		Resource: "network-attachment-definitions",
	}

	for _, tc := range testCases {
		clientset := fake.NewSimpleClientset()
		if tc.given.ipPool != nil {
			err := clientset.Tracker().Add(tc.given.ipPool)
			assert.NoError(t, err, "mock resource should add into fake controller tracker")
		}
		if tc.given.nad != nil {
			err := clientset.Tracker().Create(nadGVR, tc.given.nad, tc.given.nad.Namespace)
			assert.NoError(t, err, "mock resource should add into fake controller tracker")
		}

		ipPoolCache := fakeclient.IPPoolCache(clientset.NetworkV1alpha1().IPPools)
		nadCache := fakeclient.NetworkAttachmentDefinitionCache(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions)
		validator := NewValidator(ipPoolCache, nadCache)

		err := validator.Create(&admission.Request{}, tc.given.vmNetCfg)
		assert.Equal(t, tc.expected.shouldErr, err != nil, tc.name)
	}
}

func TestValidator_Update(t *testing.T) {
	type input struct {
		oldVmNetCfg *networkv1.VirtualMachineNetworkConfig
		vmNetCfg    *networkv1.VirtualMachineNetworkConfig
		ipPool      *networkv1.IPPool
		nad         *cniv1.NetworkAttachmentDefinition
	}

	testCases := []struct {
		name     string
		given    input
		expected bool
	}{
		{
			name: "desired static ip inside pool is allowed",
			given: input{
				oldVmNetCfg: newTestVirtualMachineNetworkConfigBuilder().
					WithNetworkConfig("", testMACAddress1, testNetworkName).Build(),
				vmNetCfg: newTestVirtualMachineNetworkConfigBuilder().
					WithNetworkConfig(testIPAddress1, testMACAddress1, testNetworkName).Build(),
				ipPool: newTestIPPoolBuilder().
					NetworkName(testNetworkName).
					CIDR(testCIDR).
					PoolRange(testStartIP, testEndIP).Build(),
				nad: newTestNetworkAttachmentDefinitionBuilder().
					Label(util.IPPoolNamespaceLabelKey, testIPPoolNamespace).
					Label(util.IPPoolNameLabelKey, testIPPoolName).Build(),
			},
			expected: false,
		},
		{
			name: "desired static ip outside pool range is denied",
			given: input{
				oldVmNetCfg: newTestVirtualMachineNetworkConfigBuilder().
					WithNetworkConfig("", testMACAddress1, testNetworkName).Build(),
				vmNetCfg: newTestVirtualMachineNetworkConfigBuilder().
					WithNetworkConfig(testOutsideRangeIP, testMACAddress1, testNetworkName).Build(),
				ipPool: newTestIPPoolBuilder().
					NetworkName(testNetworkName).
					CIDR(testCIDR).
					PoolRange(testStartIP, testEndIP).Build(),
				nad: newTestNetworkAttachmentDefinitionBuilder().
					Label(util.IPPoolNamespaceLabelKey, testIPPoolNamespace).
					Label(util.IPPoolNameLabelKey, testIPPoolName).Build(),
			},
			expected: true,
		},
		{
			name: "unchanged desired static ip does not require nad or ippool lookup",
			given: input{
				oldVmNetCfg: newTestVirtualMachineNetworkConfigBuilder().
					WithNetworkConfig(testIPAddress1, testMACAddress1, testNetworkName).Build(),
				vmNetCfg: newTestVirtualMachineNetworkConfigBuilder().
					WithNetworkConfig(testIPAddress1, testMACAddress1, testNetworkName).Build(),
			},
			expected: false,
		},
		{
			name: "removed network config does not require nad or ippool lookup",
			given: input{
				oldVmNetCfg: newTestVirtualMachineNetworkConfigBuilder().
					WithNetworkConfig(testIPAddress1, testMACAddress1, testNetworkName).Build(),
				vmNetCfg: newTestVirtualMachineNetworkConfigBuilder().Build(),
			},
			expected: false,
		},
		{
			name: "removed desired static ip does not require nad or ippool lookup",
			given: input{
				oldVmNetCfg: newTestVirtualMachineNetworkConfigBuilder().
					WithNetworkConfig(testIPAddress1, testMACAddress1, testNetworkName).Build(),
				vmNetCfg: newTestVirtualMachineNetworkConfigBuilder().
					WithNetworkConfig("", testMACAddress1, testNetworkName).Build(),
			},
			expected: false,
		},
		{
			name: "changed desired static ip requires nad and ippool lookup",
			given: input{
				oldVmNetCfg: newTestVirtualMachineNetworkConfigBuilder().
					WithNetworkConfig(testIPAddress1, testMACAddress1, testNetworkName).Build(),
				vmNetCfg: newTestVirtualMachineNetworkConfigBuilder().
					WithNetworkConfig(testIPAddress2, testMACAddress1, testNetworkName).Build(),
			},
			expected: true,
		},
		{
			name: "new dynamic network config does not require nad or ippool lookup",
			given: input{
				oldVmNetCfg: newTestVirtualMachineNetworkConfigBuilder().Build(),
				vmNetCfg: newTestVirtualMachineNetworkConfigBuilder().
					WithNetworkConfig("", testMACAddress1, testNetworkName).Build(),
			},
			expected: false,
		},
		{
			name: "changed desired static ip duplicated with unchanged config is denied",
			given: input{
				oldVmNetCfg: newTestVirtualMachineNetworkConfigBuilder().
					WithNetworkConfig("", testMACAddress1, testNetworkName).
					WithNetworkConfig(testIPAddress2, testMACAddress2, testNetworkName).Build(),
				vmNetCfg: newTestVirtualMachineNetworkConfigBuilder().
					WithNetworkConfig(testIPAddress2, testMACAddress1, testNetworkName).
					WithNetworkConfig(testIPAddress2, testMACAddress2, testNetworkName).Build(),
				ipPool: newTestIPPoolBuilder().
					NetworkName(testNetworkName).
					CIDR(testCIDR).
					PoolRange(testStartIP, testEndIP).Build(),
				nad: newTestNetworkAttachmentDefinitionBuilder().
					Label(util.IPPoolNamespaceLabelKey, testIPPoolNamespace).
					Label(util.IPPoolNameLabelKey, testIPPoolName).Build(),
			},
			expected: true,
		},
	}

	nadGVR := schema.GroupVersionResource{
		Group:    "k8s.cni.cncf.io",
		Version:  "v1",
		Resource: "network-attachment-definitions",
	}

	for _, tc := range testCases {
		clientset := fake.NewSimpleClientset()
		if tc.given.ipPool != nil {
			err := clientset.Tracker().Add(tc.given.ipPool)
			assert.NoError(t, err, "mock resource should add into fake controller tracker")
		}
		if tc.given.nad != nil {
			err := clientset.Tracker().Create(nadGVR, tc.given.nad, tc.given.nad.Namespace)
			assert.NoError(t, err, "mock resource should add into fake controller tracker")
		}

		ipPoolCache := fakeclient.IPPoolCache(clientset.NetworkV1alpha1().IPPools)
		nadCache := fakeclient.NetworkAttachmentDefinitionCache(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions)
		validator := NewValidator(ipPoolCache, nadCache)

		err := validator.Update(&admission.Request{}, tc.given.oldVmNetCfg, tc.given.vmNetCfg)
		assert.Equal(t, tc.expected, err != nil, tc.name)
	}
}

func TestValidator_Resource(t *testing.T) {
	validator := NewValidator(nil, nil)

	resource := validator.Resource()

	assert.Contains(t, resource.OperationTypes, admissionregv1.Create)
	assert.Contains(t, resource.OperationTypes, admissionregv1.Update)
}
