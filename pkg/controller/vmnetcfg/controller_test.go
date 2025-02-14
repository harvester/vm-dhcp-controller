package vmnetcfg

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	networkv1 "github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/harvester/vm-dhcp-controller/pkg/cache"
	"github.com/harvester/vm-dhcp-controller/pkg/controller/ippool"
	"github.com/harvester/vm-dhcp-controller/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/vm-dhcp-controller/pkg/ipam"
	"github.com/harvester/vm-dhcp-controller/pkg/metrics"
	"github.com/harvester/vm-dhcp-controller/pkg/util"
	"github.com/harvester/vm-dhcp-controller/pkg/util/fakeclient"
)

const (
	testNADNamespace      = "default"
	testNADName           = "net-1"
	testVmNetCfgNamespace = "default"
	testVmNetCfgName      = "test-vm"
	testKey               = testVmNetCfgNamespace + "/" + testVmNetCfgName
	testIPPoolNamespace   = "test"
	testIPPoolName        = "pool-1"

	testServerIP    = "192.168.0.2"
	testNetworkName = testNADNamespace + "/" + testNADName
	testCIDR        = "192.168.0.0/24"
	testStartIP     = "192.168.0.101"
	testEndIP       = "192.168.0.200"

	testIPAddress1  = "192.168.0.111"
	testIPAddress2  = "192.168.0.177"
	testMACAddress1 = "11:22:33:44:55:66"
	testMACAddress2 = "22:33:44:55:66:77"
)

func newTestVmNetCfgBuilder() *vmNetCfgBuilder {
	return newVmNetCfgBuilder(testVmNetCfgNamespace, testVmNetCfgName)
}

func newTestVmNetCfgStatusBuilder() *vmNetCfgStatusBuilder {
	return newVmNetCfgStatusBuilder()
}

func newTestIPPoolBuilder() *ippool.IPPoolBuilder {
	return ippool.NewIPPoolBuilder(testIPPoolNamespace, testIPPoolName)
}

func newTestCacheAllocatorBuilder() *cache.CacheAllocatorBuilder {
	return cache.NewCacheAllocatorBuilder()
}

func newTestIPAllocatorBuilder() *ipam.IPAllocatorBuilder {
	return ipam.NewIPAllocatorBuilder()
}

func newTestNetworkAttachmentDefinitionBuilder() *ippool.NetworkAttachmentDefinitionBuilder {
	return ippool.NewNetworkAttachmentDefinitionBuilder(testNADNamespace, testNADName)
}

func TestHandler_OnChange(t *testing.T) {
	t.Run("new vmnetcfg", func(t *testing.T) {
		givenVmNetCfg := newTestVmNetCfgBuilder().Build()

		expectedVmNetCfg := newTestVmNetCfgBuilder().
			DisabledCondition(corev1.ConditionFalse, "", "").Build()

		clientset := fake.NewSimpleClientset()
		err := clientset.Tracker().Add(givenVmNetCfg)
		if err != nil {
			t.Fatal(err)
		}

		handler := Handler{
			vmnetcfgClient: fakeclient.VirtualMachineNetworkConfigClient(clientset.NetworkV1alpha1().VirtualMachineNetworkConfigs),
		}

		vmNetCfg, err := handler.OnChange(testVmNetCfgNamespace+"/"+testVmNetCfgName, givenVmNetCfg)

		assert.Nil(t, err)

		SanitizeStatus(&expectedVmNetCfg.Status)
		SanitizeStatus(&vmNetCfg.Status)

		assert.Equal(t, expectedVmNetCfg, vmNetCfg)
	})

	t.Run("pause vmnetcfg", func(t *testing.T) {
		givenVmNetCfg := newTestVmNetCfgBuilder().
			Paused().Build()

		expectedVmNetCfg := newTestVmNetCfgBuilder().
			Paused().
			DisabledCondition(corev1.ConditionTrue, "", "").Build()

		clientset := fake.NewSimpleClientset()
		err := clientset.Tracker().Add(givenVmNetCfg)
		if err != nil {
			t.Fatal(err)
		}

		handler := Handler{
			metricsAllocator: metrics.New(),
			vmnetcfgClient:   fakeclient.VirtualMachineNetworkConfigClient(clientset.NetworkV1alpha1().VirtualMachineNetworkConfigs),
		}

		vmNetCfg, err := handler.OnChange(testVmNetCfgNamespace+"/"+testVmNetCfgName, givenVmNetCfg)

		assert.Nil(t, err)

		SanitizeStatus(&expectedVmNetCfg.Status)
		SanitizeStatus(&vmNetCfg.Status)

		assert.Equal(t, expectedVmNetCfg, vmNetCfg)
	})

	t.Run("pause vmnetcfg with ips allocated", func(t *testing.T) {
		givenVmNetCfg := newTestVmNetCfgBuilder().
			Paused().
			WithNetworkConfig(testIPAddress1, testMACAddress1, testNetworkName).
			WithNetworkConfig(testIPAddress2, testMACAddress2, testNetworkName).
			WithNetworkConfigStatus(testIPAddress1, testMACAddress1, testNetworkName, networkv1.AllocatedState).
			WithNetworkConfigStatus(testIPAddress2, testMACAddress2, testNetworkName, networkv1.AllocatedState).
			AllocatedCondition(corev1.ConditionTrue, "", "").Build()
		givenIPPool := newTestIPPoolBuilder().
			ServerIP(testServerIP).
			CIDR(testCIDR).
			PoolRange(testStartIP, testEndIP).
			NetworkName(testNetworkName).
			Allocated(testIPAddress1, testMACAddress1).Build()
		givenIPAllocator := newTestIPAllocatorBuilder().
			IPSubnet(testNetworkName, testCIDR, testStartIP, testEndIP).
			Allocate(testNetworkName, testIPAddress1).Build()
		givenCacheAllocator := newTestCacheAllocatorBuilder().
			MACSet(testNetworkName).
			Add(testNetworkName, testMACAddress1, testIPAddress1).Build()
		givenNAD := newTestNetworkAttachmentDefinitionBuilder().
			Label(util.IPPoolNamespaceLabelKey, testIPPoolNamespace).
			Label(util.IPPoolNameLabelKey, testIPPoolName).Build()

		expectedVmNetCfg := newTestVmNetCfgBuilder().
			Paused().
			WithNetworkConfig(testIPAddress1, testMACAddress1, testNetworkName).
			WithNetworkConfig(testIPAddress2, testMACAddress2, testNetworkName).
			WithNetworkConfigStatus(testIPAddress1, testMACAddress1, testNetworkName, networkv1.PendingState).
			WithNetworkConfigStatus(testIPAddress2, testMACAddress2, testNetworkName, networkv1.PendingState).
			AllocatedCondition(corev1.ConditionTrue, "", "").
			DisabledCondition(corev1.ConditionTrue, "", "").Build()

		nadGVR := schema.GroupVersionResource{
			Group:    "k8s.cni.cncf.io",
			Version:  "v1",
			Resource: "network-attachment-definitions",
		}

		clientset := fake.NewSimpleClientset()
		err := clientset.Tracker().Create(nadGVR, givenNAD, givenNAD.Namespace)
		assert.Nil(t, err, "mock resource should add into fake controller tracker")

		err = clientset.Tracker().Add(givenVmNetCfg)
		if err != nil {
			t.Fatal(err)
		}
		err = clientset.Tracker().Add(givenIPPool)
		if err != nil {
			t.Fatal(err)
		}

		handler := Handler{
			cacheAllocator:   givenCacheAllocator,
			ipAllocator:      givenIPAllocator,
			metricsAllocator: metrics.New(),
			vmnetcfgClient:   fakeclient.VirtualMachineNetworkConfigClient(clientset.NetworkV1alpha1().VirtualMachineNetworkConfigs),
			ippoolClient:     fakeclient.IPPoolClient(clientset.NetworkV1alpha1().IPPools),
			ippoolCache:      fakeclient.IPPoolCache(clientset.NetworkV1alpha1().IPPools),
			nadCache:         fakeclient.NetworkAttachmentDefinitionCache(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions),
		}

		vmNetCfg, err := handler.OnChange(testKey, givenVmNetCfg)

		assert.Nil(t, err)

		SanitizeStatus(&expectedVmNetCfg.Status)
		SanitizeStatus(&vmNetCfg.Status)

		assert.Equal(t, expectedVmNetCfg, vmNetCfg)
	})
}

func TestHandler_Allocate(t *testing.T) {
	t.Run("new vmnetcfg", func(t *testing.T) {
		givenVmNetCfg := newTestVmNetCfgBuilder().
			WithNetworkConfig(testIPAddress1, testMACAddress1, testNetworkName).
			WithNetworkConfig(testIPAddress2, testMACAddress2, testNetworkName).Build()
		givenIPPool := newTestIPPoolBuilder().
			ServerIP(testServerIP).
			CIDR(testCIDR).
			PoolRange(testStartIP, testEndIP).
			NetworkName(testNetworkName).
			CacheReadyCondition(corev1.ConditionTrue, "", "").Build()
		givenCacheAllocator := newTestCacheAllocatorBuilder().
			MACSet(testNetworkName).Build()
		givenIPAllocator := newTestIPAllocatorBuilder().
			IPSubnet(testNetworkName, testCIDR, testStartIP, testEndIP).Build()
		givenNAD := newTestNetworkAttachmentDefinitionBuilder().
			Label(util.IPPoolNamespaceLabelKey, testIPPoolNamespace).
			Label(util.IPPoolNameLabelKey, testIPPoolName).Build()

		expectedStatus := newTestVmNetCfgStatusBuilder().
			WithNetworkConfigStatus(testIPAddress1, testMACAddress1, testNetworkName, networkv1.AllocatedState).
			WithNetworkConfigStatus(testIPAddress2, testMACAddress2, testNetworkName, networkv1.AllocatedState).Build()
		expectedIPPool := newTestIPPoolBuilder().
			ServerIP(testServerIP).
			CIDR(testCIDR).
			PoolRange(testStartIP, testEndIP).
			NetworkName(testNetworkName).
			Allocated(testIPAddress1, testMACAddress1).
			Allocated(testIPAddress2, testMACAddress2).
			CacheReadyCondition(corev1.ConditionTrue, "", "").Build()
		expectedCacheAllocator := newTestCacheAllocatorBuilder().
			MACSet(testNetworkName).
			Add(testNetworkName, testMACAddress1, testIPAddress1).
			Add(testNetworkName, testMACAddress2, testIPAddress2).Build()
		expectedIPAllocator := newTestIPAllocatorBuilder().
			IPSubnet(testNetworkName, testCIDR, testStartIP, testEndIP).
			Allocate(testNetworkName, testIPAddress1, testIPAddress2).Build()

		nadGVR := schema.GroupVersionResource{
			Group:    "k8s.cni.cncf.io",
			Version:  "v1",
			Resource: "network-attachment-definitions",
		}

		clientset := fake.NewSimpleClientset()
		err := clientset.Tracker().Create(nadGVR, givenNAD, givenNAD.Namespace)
		assert.Nil(t, err, "mock resource should add into fake controller tracker")

		err = clientset.Tracker().Add(givenVmNetCfg)
		if err != nil {
			t.Fatal(err)
		}
		err = clientset.Tracker().Add(givenIPPool)
		if err != nil {
			t.Fatal(err)
		}

		handler := Handler{
			cacheAllocator:   givenCacheAllocator,
			ipAllocator:      givenIPAllocator,
			metricsAllocator: metrics.New(),
			ippoolClient:     fakeclient.IPPoolClient(clientset.NetworkV1alpha1().IPPools),
			ippoolCache:      fakeclient.IPPoolCache(clientset.NetworkV1alpha1().IPPools),
			nadCache:         fakeclient.NetworkAttachmentDefinitionCache(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions),
		}

		status, err := handler.Allocate(givenVmNetCfg, givenVmNetCfg.Status)
		assert.Nil(t, err)

		SanitizeStatus(&expectedStatus)
		SanitizeStatus(&status)
		assert.Equal(t, expectedStatus, status)

		ipPool, err := handler.ippoolClient.Get(testIPPoolNamespace, testIPPoolName, metav1.GetOptions{})
		assert.Nil(t, err)

		ippool.SanitizeStatus(&expectedIPPool.Status)
		ippool.SanitizeStatus(&ipPool.Status)
		assert.Equal(t, expectedIPPool, ipPool)

		assert.Equal(t, expectedIPAllocator, handler.ipAllocator)
		assert.Equal(t, expectedCacheAllocator, handler.cacheAllocator)
	})

	t.Run("rebuild caches", func(t *testing.T) {
		givenVmNetCfg := newTestVmNetCfgBuilder().
			WithNetworkConfig(testIPAddress1, testMACAddress1, testNetworkName).
			WithNetworkConfig(testIPAddress2, testMACAddress2, testNetworkName).
			WithNetworkConfigStatus(testIPAddress1, testMACAddress1, testNetworkName, networkv1.AllocatedState).
			WithNetworkConfigStatus(testIPAddress1, testMACAddress1, testNetworkName, networkv1.AllocatedState).Build()
		givenIPPool := newTestIPPoolBuilder().
			ServerIP(testServerIP).
			CIDR(testCIDR).
			PoolRange(testStartIP, testEndIP).
			NetworkName(testNetworkName).
			Allocated(testIPAddress1, testMACAddress1).
			Allocated(testIPAddress2, testMACAddress2).
			CacheReadyCondition(corev1.ConditionTrue, "", "").Build()
		givenCacheAllocator := newTestCacheAllocatorBuilder().
			MACSet(testNetworkName).Build()
		givenIPAllocator := newTestIPAllocatorBuilder().
			IPSubnet(testNetworkName, testCIDR, testStartIP, testEndIP).Build()
		givenNAD := newTestNetworkAttachmentDefinitionBuilder().
			Label(util.IPPoolNamespaceLabelKey, testIPPoolNamespace).
			Label(util.IPPoolNameLabelKey, testIPPoolName).Build()

		expectedCacheAllocator := newTestCacheAllocatorBuilder().
			MACSet(testNetworkName).
			Add(testNetworkName, testMACAddress1, testIPAddress1).
			Add(testNetworkName, testMACAddress2, testIPAddress2).Build()
		expectedIPAllocator := newTestIPAllocatorBuilder().
			IPSubnet(testNetworkName, testCIDR, testStartIP, testEndIP).
			Allocate(testNetworkName, testIPAddress1, testIPAddress2).Build()

		nadGVR := schema.GroupVersionResource{
			Group:    "k8s.cni.cncf.io",
			Version:  "v1",
			Resource: "network-attachment-definitions",
		}

		clientset := fake.NewSimpleClientset()
		err := clientset.Tracker().Create(nadGVR, givenNAD, givenNAD.Namespace)
		assert.Nil(t, err, "mock resource should add into fake controller tracker")

		err = clientset.Tracker().Add(givenIPPool)
		if err != nil {
			t.Fatal(err)
		}

		handler := Handler{
			cacheAllocator:   givenCacheAllocator,
			ipAllocator:      givenIPAllocator,
			metricsAllocator: metrics.New(),
			ippoolClient:     fakeclient.IPPoolClient(clientset.NetworkV1alpha1().IPPools),
			ippoolCache:      fakeclient.IPPoolCache(clientset.NetworkV1alpha1().IPPools),
			nadCache:         fakeclient.NetworkAttachmentDefinitionCache(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions),
		}

		_, err = handler.Allocate(givenVmNetCfg, givenVmNetCfg.Status)
		assert.Nil(t, err)

		assert.Equal(t, expectedIPAllocator, handler.ipAllocator)
		assert.Equal(t, expectedCacheAllocator, handler.cacheAllocator)
	})

	t.Run("pause vmnetcfg", func(t *testing.T) {
		givenVmNetCfg := newTestVmNetCfgBuilder().
			Paused().
			WithNetworkConfig(testIPAddress1, testMACAddress1, testNetworkName).
			WithNetworkConfig(testIPAddress2, testMACAddress2, testNetworkName).
			WithNetworkConfigStatus(testIPAddress1, testMACAddress1, testNetworkName, networkv1.AllocatedState).
			WithNetworkConfigStatus(testIPAddress1, testMACAddress1, testNetworkName, networkv1.AllocatedState).Build()

		handler := Handler{}

		_, err := handler.Allocate(givenVmNetCfg, givenVmNetCfg.Status)
		assert.Equal(t, fmt.Sprintf("vmnetcfg %s/%s was administratively disabled", testVmNetCfgNamespace, testVmNetCfgName), err.Error())
	})

	t.Run("recover ips from cache", func(t *testing.T) {
		givenVmNetCfg := newTestVmNetCfgBuilder().
			WithNetworkConfig(testIPAddress1, testMACAddress1, testNetworkName).
			WithNetworkConfig(testIPAddress2, testMACAddress2, testNetworkName).Build()
		givenIPPool := newTestIPPoolBuilder().
			ServerIP(testServerIP).
			CIDR(testCIDR).
			PoolRange(testStartIP, testEndIP).
			NetworkName(testNetworkName).
			CacheReadyCondition(corev1.ConditionTrue, "", "").Build()
		givenCacheAllocator := newTestCacheAllocatorBuilder().
			MACSet(testNetworkName).
			Add(testNetworkName, testMACAddress1, testIPAddress1).
			Add(testNetworkName, testMACAddress2, testIPAddress2).Build()
		givenNAD := newTestNetworkAttachmentDefinitionBuilder().
			Label(util.IPPoolNamespaceLabelKey, testIPPoolNamespace).
			Label(util.IPPoolNameLabelKey, testIPPoolName).Build()

		expectedStatus := newTestVmNetCfgStatusBuilder().
			WithNetworkConfigStatus(testIPAddress1, testMACAddress1, testNetworkName, networkv1.AllocatedState).
			WithNetworkConfigStatus(testIPAddress2, testMACAddress2, testNetworkName, networkv1.AllocatedState).Build()
		expectedIPPool := newTestIPPoolBuilder().
			ServerIP(testServerIP).
			CIDR(testCIDR).
			PoolRange(testStartIP, testEndIP).
			NetworkName(testNetworkName).
			Allocated(testIPAddress1, testMACAddress1).
			Allocated(testIPAddress2, testMACAddress2).
			CacheReadyCondition(corev1.ConditionTrue, "", "").Build()

		nadGVR := schema.GroupVersionResource{
			Group:    "k8s.cni.cncf.io",
			Version:  "v1",
			Resource: "network-attachment-definitions",
		}

		clientset := fake.NewSimpleClientset()
		err := clientset.Tracker().Create(nadGVR, givenNAD, givenNAD.Namespace)
		assert.Nil(t, err, "mock resource should add into fake controller tracker")

		err = clientset.Tracker().Add(givenIPPool)
		if err != nil {
			t.Fatal(err)
		}

		handler := Handler{
			cacheAllocator:   givenCacheAllocator,
			metricsAllocator: metrics.New(),
			ippoolClient:     fakeclient.IPPoolClient(clientset.NetworkV1alpha1().IPPools),
			ippoolCache:      fakeclient.IPPoolCache(clientset.NetworkV1alpha1().IPPools),
			nadCache:         fakeclient.NetworkAttachmentDefinitionCache(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions),
		}

		status, err := handler.Allocate(givenVmNetCfg, givenVmNetCfg.Status)
		assert.Nil(t, err)

		SanitizeStatus(&expectedStatus)
		SanitizeStatus(&status)
		assert.Equal(t, expectedStatus, status)

		ipPool, err := handler.ippoolClient.Get(testIPPoolNamespace, testIPPoolName, metav1.GetOptions{})
		assert.Nil(t, err)

		ippool.SanitizeStatus(&expectedIPPool.Status)
		ippool.SanitizeStatus(&ipPool.Status)

		assert.Equal(t, expectedIPPool, ipPool)
	})

	t.Run("ippool cache not ready", func(t *testing.T) {
		givenVmNetCfg := newTestVmNetCfgBuilder().
			WithNetworkConfig(testIPAddress1, testMACAddress1, testNetworkName).
			WithNetworkConfig(testIPAddress2, testMACAddress2, testNetworkName).Build()
		givenIPPool := newTestIPPoolBuilder().
			ServerIP(testServerIP).
			CIDR(testCIDR).
			PoolRange(testStartIP, testEndIP).
			NetworkName(testNetworkName).
			CacheReadyCondition(corev1.ConditionFalse, "", "").Build()
		givenNAD := newTestNetworkAttachmentDefinitionBuilder().
			Label(util.IPPoolNamespaceLabelKey, testIPPoolNamespace).
			Label(util.IPPoolNameLabelKey, testIPPoolName).Build()

		nadGVR := schema.GroupVersionResource{
			Group:    "k8s.cni.cncf.io",
			Version:  "v1",
			Resource: "network-attachment-definitions",
		}

		clientset := fake.NewSimpleClientset()
		err := clientset.Tracker().Create(nadGVR, givenNAD, givenNAD.Namespace)
		assert.Nil(t, err, "mock resource should add into fake controller tracker")

		err = clientset.Tracker().Add(givenIPPool)
		if err != nil {
			t.Fatal(err)
		}

		handler := Handler{
			ippoolClient: fakeclient.IPPoolClient(clientset.NetworkV1alpha1().IPPools),
			ippoolCache:  fakeclient.IPPoolCache(clientset.NetworkV1alpha1().IPPools),
			nadCache:     fakeclient.NetworkAttachmentDefinitionCache(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions),
		}

		_, err = handler.Allocate(givenVmNetCfg, givenVmNetCfg.Status)
		assert.NotNil(t, fmt.Sprintf("ippool %s/%s is not ready", testIPPoolNamespace, testIPPoolName), err)
	})

	t.Run("ippool not found", func(t *testing.T) {
		givenVmNetCfg := newTestVmNetCfgBuilder().
			WithNetworkConfig(testIPAddress1, testMACAddress1, testNetworkName).
			WithNetworkConfig(testIPAddress2, testMACAddress2, testNetworkName).Build()
		givenNAD := newTestNetworkAttachmentDefinitionBuilder().
			Label(util.IPPoolNamespaceLabelKey, testIPPoolNamespace).
			Label(util.IPPoolNameLabelKey, testIPPoolName).Build()

		nadGVR := schema.GroupVersionResource{
			Group:    "k8s.cni.cncf.io",
			Version:  "v1",
			Resource: "network-attachment-definitions",
		}

		clientset := fake.NewSimpleClientset()
		err := clientset.Tracker().Create(nadGVR, givenNAD, givenNAD.Namespace)
		assert.Nil(t, err, "mock resource should add into fake controller tracker")

		handler := Handler{
			ippoolClient: fakeclient.IPPoolClient(clientset.NetworkV1alpha1().IPPools),
			ippoolCache:  fakeclient.IPPoolCache(clientset.NetworkV1alpha1().IPPools),
			nadCache:     fakeclient.NetworkAttachmentDefinitionCache(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions),
		}

		_, err = handler.Allocate(givenVmNetCfg, givenVmNetCfg.Status)
		assert.NotNil(t, fmt.Sprintf("ippool %s/%s not found", testIPPoolNamespace, testIPPoolName), err)
	})

	t.Run("nad not labeled with ippool info", func(t *testing.T) {
		givenVmNetCfg := newTestVmNetCfgBuilder().
			WithNetworkConfig(testIPAddress1, testMACAddress1, testNetworkName).
			WithNetworkConfig(testIPAddress2, testMACAddress2, testNetworkName).Build()
		givenIPPool := newTestIPPoolBuilder().
			ServerIP(testServerIP).
			CIDR(testCIDR).
			PoolRange(testStartIP, testEndIP).
			NetworkName(testNetworkName).
			CacheReadyCondition(corev1.ConditionTrue, "", "").Build()
		givenNAD := newTestNetworkAttachmentDefinitionBuilder().Build()

		nadGVR := schema.GroupVersionResource{
			Group:    "k8s.cni.cncf.io",
			Version:  "v1",
			Resource: "network-attachment-definitions",
		}

		clientset := fake.NewSimpleClientset()
		err := clientset.Tracker().Create(nadGVR, givenNAD, givenNAD.Namespace)
		assert.Nil(t, err, "mock resource should add into fake controller tracker")

		err = clientset.Tracker().Add(givenIPPool)
		if err != nil {
			t.Fatal(err)
		}

		handler := Handler{
			ippoolClient: fakeclient.IPPoolClient(clientset.NetworkV1alpha1().IPPools),
			ippoolCache:  fakeclient.IPPoolCache(clientset.NetworkV1alpha1().IPPools),
			nadCache:     fakeclient.NetworkAttachmentDefinitionCache(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions),
		}

		_, err = handler.Allocate(givenVmNetCfg, givenVmNetCfg.Status)
		assert.NotNil(t, fmt.Errorf("network attachment definition %s/%s has no labels", testNADNamespace, testNADName), err)
	})
}
