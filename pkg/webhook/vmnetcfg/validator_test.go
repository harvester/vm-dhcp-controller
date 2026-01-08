package vmnetcfg

import (
	"testing"

	"github.com/harvester/webhook/pkg/server/admission"
	cniv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/stretchr/testify/assert"
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
