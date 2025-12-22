package ippool

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/harvester/vm-dhcp-controller/pkg/cache"
	"github.com/harvester/vm-dhcp-controller/pkg/config"
	"github.com/harvester/vm-dhcp-controller/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/vm-dhcp-controller/pkg/ipam"
	"github.com/harvester/vm-dhcp-controller/pkg/metrics"
	"github.com/harvester/vm-dhcp-controller/pkg/util"
	"github.com/harvester/vm-dhcp-controller/pkg/util/fakeclient"
)

const (
	testNADNamespace        = "default"
	testNADName             = "net-1"
	testNADNameLong         = "fi6cx9ca1kt1faq80k3ro9cowyumyjb67qdmg8fb9ydmz27rbk5btlg2m5avv3n"
	testIPPoolNamespace     = testNADNamespace
	testIPPoolName          = testNADName
	testIPPoolNameLong      = testNADNameLong
	testKey                 = testIPPoolNamespace + "/" + testIPPoolName
	testDeploymentNamespace = "harvester-system"
	testDeploymentName      = testNADNamespace + "-" + testNADName + "-agent"
	testUID                 = "3a955369-9eaa-43db-94f3-9153289d7dc2"
	testClusterNetwork      = "provider"
	testServerIP1           = "192.168.0.2"
	testServerIP2           = "192.168.0.110"
	testNetworkName         = testNADNamespace + "/" + testNADName
	testNetworkNameLong     = testNADNamespace + "/" + testNADNameLong
	testCIDR                = "192.168.0.0/24"
	testRouter1             = "192.168.0.1"
	testRouter2             = "192.168.0.120"
	testStartIP             = "192.168.0.101"
	testEndIP               = "192.168.0.200"
	testServiceAccountName  = "vdca"
	testImageRepository     = "rancher/harvester-vm-dhcp-agent"
	testImageTag            = "main"
	testImageTagNew         = "dev"
	testImage               = testImageRepository + ":" + testImageTag
	testImageNew            = testImageRepository + ":" + testImageTagNew
	testContainerName       = "agent"

	testExcludedIP1 = "192.168.0.150"
	testExcludedIP2 = "192.168.0.187"
	testExcludedIP3 = "192.168.0.10"
	testExcludedIP4 = "192.168.0.235"

	testAllocatedIP1 = "192.168.0.111"
	testAllocatedIP2 = "192.168.0.177"
	testMAC1         = "11:22:33:44:55:66"
	testMAC2         = "22:33:44:55:66:77"
)

var (
	testDeploymentNameLong = util.SafeAgentConcatName(testNADNamespace, testNADNameLong)
)

func newTestCacheAllocatorBuilder() *cache.CacheAllocatorBuilder {
	return cache.NewCacheAllocatorBuilder()
}

func newTestIPAllocatorBuilder() *ipam.IPAllocatorBuilder {
	return ipam.NewIPAllocatorBuilder()
}

func newTestIPPoolBuilder() *IPPoolBuilder {
	return NewIPPoolBuilder(testIPPoolNamespace, testIPPoolName)
}

func newTestDeploymentBuilder() *deploymentBuilder {
	return newDeploymentBuilder(testDeploymentNamespace, testDeploymentName)
}

func newTestIPPoolStatusBuilder() *ipPoolStatusBuilder {
	return newIPPoolStatusBuilder()
}

func newTestNetworkAttachmentDefinitionBuilder() *NetworkAttachmentDefinitionBuilder {
	return NewNetworkAttachmentDefinitionBuilder(testNADNamespace, testNADName)
}

func TestHandler_OnChange(t *testing.T) {
	t.Run("new ippool", func(t *testing.T) {
		key := testIPPoolNamespace + "/" + testIPPoolName
		givenIPAllocator := newTestIPAllocatorBuilder().Build()
		givenIPPool := newTestIPPoolBuilder().
			NetworkName(testNetworkName).Build()
		givenNAD := newTestNetworkAttachmentDefinitionBuilder().Build()

		expectedIPPool := newTestIPPoolBuilder().
			NetworkName(testNetworkName).
			StoppedCondition(corev1.ConditionFalse, "", "").
			CacheReadyCondition(corev1.ConditionFalse, "NotInitialized", "").Build()
		expectedNAD := newTestNetworkAttachmentDefinitionBuilder().
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
			agentNamespace: "default",
			agentImage: &config.Image{
				Repository: "rancher/harvester-vm-dhcp-controller",
				Tag:        "main",
			},
			ipAllocator:  givenIPAllocator,
			ippoolClient: fakeclient.IPPoolClient(clientset.NetworkV1alpha1().IPPools),
			nadClient:    fakeclient.NetworkAttachmentDefinitionClient(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions),
			nadCache:     fakeclient.NetworkAttachmentDefinitionCache(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions),
		}

		ipPool, err := handler.OnChange(key, givenIPPool)
		assert.Nil(t, err)

		SanitizeStatus(&expectedIPPool.Status)
		SanitizeStatus(&ipPool.Status)

		assert.Equal(t, expectedIPPool, ipPool)

		nad, err := handler.nadClient.Get(testNADNamespace, testNADName, metav1.GetOptions{})
		assert.Nil(t, err)
		assert.Equal(t, expectedNAD, nad)
	})

	t.Run("ippool with ipam initialized", func(t *testing.T) {
		key := testIPPoolNamespace + "/" + testIPPoolName
		givenIPAllocator := newTestIPAllocatorBuilder().
			IPSubnet(testNetworkName, testCIDR, testStartIP, testEndIP).
			Build()
		givenIPPool := newTestIPPoolBuilder().
			ServerIP(testServerIP1).
			CIDR(testCIDR).
			PoolRange(testStartIP, testEndIP).
			NetworkName(testNetworkName).
			CacheReadyCondition(corev1.ConditionTrue, "", "").Build()
		givenNAD := newTestNetworkAttachmentDefinitionBuilder().Build()

		expectedIPPool := newTestIPPoolBuilder().
			ServerIP(testServerIP1).
			CIDR(testCIDR).
			PoolRange(testStartIP, testEndIP).
			NetworkName(testNetworkName).
			Available(100).
			Used(0).
			CacheReadyCondition(corev1.ConditionTrue, "", "").
			StoppedCondition(corev1.ConditionFalse, "", "").Build()

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
			agentNamespace: "default",
			agentImage: &config.Image{
				Repository: "rancher/harvester-vm-dhcp-controller",
				Tag:        "main",
			},
			ipAllocator:      givenIPAllocator,
			metricsAllocator: metrics.New(),
			ippoolClient:     fakeclient.IPPoolClient(clientset.NetworkV1alpha1().IPPools),
			nadClient:        fakeclient.NetworkAttachmentDefinitionClient(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions),
			nadCache:         fakeclient.NetworkAttachmentDefinitionCache(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions),
		}

		ipPool, err := handler.OnChange(key, givenIPPool)
		assert.Nil(t, err)

		SanitizeStatus(&expectedIPPool.Status)
		SanitizeStatus(&ipPool.Status)

		assert.Equal(t, expectedIPPool, ipPool)
	})

	t.Run("pause ippool", func(t *testing.T) {
		key := testIPPoolNamespace + "/" + testIPPoolName
		givenIPAllocator := newTestIPAllocatorBuilder().
			IPSubnet(testNetworkName, testCIDR, testStartIP, testEndIP).Build()
		givenIPPool := newTestIPPoolBuilder().
			NetworkName(testNetworkName).
			Paused().
			AgentDeploymentRef(testDeploymentNamespace, testDeploymentName, testImage, "").Build()
		givenDeployment := newTestDeploymentBuilder().Build()
		givenNAD := newTestNetworkAttachmentDefinitionBuilder().Build()

		expectedIPAllocator := newTestIPAllocatorBuilder().Build()
		expectedIPPool := newTestIPPoolBuilder().
			NetworkName(testNetworkName).
			Paused().
			StoppedCondition(corev1.ConditionTrue, "", "").Build()

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

		k8sclientset := k8sfake.NewSimpleClientset()
		err = k8sclientset.Tracker().Add(givenDeployment)
		assert.Nil(t, err, "mock resource should add into fake controller tracker")

		handler := Handler{
			agentNamespace: "default",
			agentImage: &config.Image{
				Repository: "rancher/harvester-vm-dhcp-controller",
				Tag:        "main",
			},
			ipAllocator:      givenIPAllocator,
			cacheAllocator:   cache.New(),
			metricsAllocator: metrics.New(),
			ippoolClient:     fakeclient.IPPoolClient(clientset.NetworkV1alpha1().IPPools),
			deploymentClient: fakeclient.DeploymentClient(k8sclientset.AppsV1().Deployments),
			nadClient:        fakeclient.NetworkAttachmentDefinitionClient(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions),
			nadCache:         fakeclient.NetworkAttachmentDefinitionCache(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions),
		}

		ipPool, err := handler.OnChange(key, givenIPPool)
		assert.Nil(t, err)

		SanitizeStatus(&expectedIPPool.Status)
		SanitizeStatus(&ipPool.Status)

		assert.Equal(t, expectedIPPool, ipPool)

		assert.Equal(t, expectedIPAllocator, handler.ipAllocator)

		_, err = handler.deploymentClient.Get(testDeploymentNamespace, testDeploymentName, metav1.GetOptions{})
		assert.Equal(t, fmt.Sprintf("deployments.apps \"%s\" not found", testDeploymentName), err.Error())

	})

	t.Run("resume ippool", func(t *testing.T) {
		key := testIPPoolNamespace + "/" + testIPPoolName
		givenIPAllocator := newTestIPAllocatorBuilder().
			IPSubnet(testNetworkName, testCIDR, testStartIP, testEndIP).
			Build()
		givenIPPool := newTestIPPoolBuilder().
			NetworkName(testNetworkName).
			UnPaused().
			CacheReadyCondition(corev1.ConditionTrue, "", "").Build()
		givenNAD := newTestNetworkAttachmentDefinitionBuilder().Build()

		expectedIPPool := newTestIPPoolBuilder().
			NetworkName(testNetworkName).
			UnPaused().
			Available(100).
			Used(0).
			CacheReadyCondition(corev1.ConditionTrue, "", "").
			StoppedCondition(corev1.ConditionFalse, "", "").Build()

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
			agentNamespace: "default",
			agentImage: &config.Image{
				Repository: "rancher/harvester-vm-dhcp-controller",
				Tag:        "main",
			},
			ipAllocator:      givenIPAllocator,
			metricsAllocator: metrics.New(),
			ippoolClient:     fakeclient.IPPoolClient(clientset.NetworkV1alpha1().IPPools),
			nadClient:        fakeclient.NetworkAttachmentDefinitionClient(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions),
			nadCache:         fakeclient.NetworkAttachmentDefinitionCache(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions),
		}

		ipPool, err := handler.OnChange(key, givenIPPool)
		assert.Nil(t, err)

		SanitizeStatus(&expectedIPPool.Status)
		SanitizeStatus(&ipPool.Status)

		assert.Equal(t, expectedIPPool, ipPool)
	})
}

func TestHandler_DeployAgent(t *testing.T) {
	t.Run("ippool created", func(t *testing.T) {
		givenIPPool := newTestIPPoolBuilder().
			ServerIP(testServerIP1).
			CIDR(testCIDR).
			NetworkName(testNetworkName).Build()
		givenNAD := newTestNetworkAttachmentDefinitionBuilder().
			Label(clusterNetworkLabelKey, testClusterNetwork).Build()

		expectedStatus := newTestIPPoolStatusBuilder().
			AgentDeploymentRef(testDeploymentNamespace, testDeploymentName, testImage, "").Build()
		expectedDeployment, _ := prepareAgentDeployment(
			NewIPPoolBuilder(testIPPoolNamespace, testIPPoolName).
				ServerIP(testServerIP1).
				CIDR(testCIDR).
				NetworkName(testNetworkName).Build(),
			false,
			testDeploymentNamespace,
			testClusterNetwork,
			testServiceAccountName,
			testImage,
		)

		nadGVR := schema.GroupVersionResource{
			Group:    "k8s.cni.cncf.io",
			Version:  "v1",
			Resource: "network-attachment-definitions",
		}

		clientset := fake.NewSimpleClientset()
		err := clientset.Tracker().Create(nadGVR, givenNAD, givenNAD.Namespace)
		assert.Nil(t, err, "mock resource should add into fake controller tracker")

		k8sclientset := k8sfake.NewSimpleClientset()

		handler := Handler{
			agentNamespace: testDeploymentNamespace,
			agentImage: &config.Image{
				Repository: testImageRepository,
				Tag:        testImageTag,
			},
			agentServiceAccountName: testServiceAccountName,
			nadCache:                fakeclient.NetworkAttachmentDefinitionCache(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions),
			deploymentClient:        fakeclient.DeploymentClient(k8sclientset.AppsV1().Deployments),
			deploymentCache:         fakeclient.DeploymentCache(k8sclientset.AppsV1().Deployments),
		}

		status, err := handler.DeployAgent(givenIPPool, givenIPPool.Status)
		assert.Nil(t, err)
		assert.Equal(t, expectedStatus, status)

		deployment, err := handler.deploymentClient.Get(testDeploymentNamespace, testDeploymentName, metav1.GetOptions{})
		assert.Nil(t, err)

		assert.Equal(t, expectedDeployment, deployment)
	})

	t.Run("ippool paused", func(t *testing.T) {
		givenIPPool := newTestIPPoolBuilder().
			Paused().Build()

		handler := Handler{
			agentNamespace: testDeploymentNamespace,
			agentImage: &config.Image{
				Repository: testImageRepository,
				Tag:        testImageTag,
			},
			agentServiceAccountName: testServiceAccountName,
		}

		_, err := handler.DeployAgent(givenIPPool, givenIPPool.Status)
		assert.Equal(t, fmt.Errorf("ippool %s was administratively disabled", testIPPoolNamespace+"/"+testIPPoolName), err)
	})

	t.Run("nad not found", func(t *testing.T) {
		givenIPPool := newTestIPPoolBuilder().
			NetworkName("you-cant-find-me").Build()
		givenNAD := newTestNetworkAttachmentDefinitionBuilder().
			Label(clusterNetworkLabelKey, testClusterNetwork).Build()

		nadGVR := schema.GroupVersionResource{
			Group:    "k8s.cni.cncf.io",
			Version:  "v1",
			Resource: "network-attachment-definitions",
		}

		clientset := fake.NewSimpleClientset()
		err := clientset.Tracker().Create(nadGVR, givenNAD, givenNAD.Namespace)
		assert.Nil(t, err, "mock resource should add into fake controller tracker")

		handler := Handler{
			nadCache: fakeclient.NetworkAttachmentDefinitionCache(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions),
		}

		_, err = handler.DeployAgent(givenIPPool, givenIPPool.Status)
		assert.Equal(t, fmt.Sprintf("network-attachment-definitions.k8s.cni.cncf.io \"%s\" not found", "you-cant-find-me"), err.Error())
	})

	t.Run("agent deployment already exists", func(t *testing.T) {
		givenIPPool := newTestIPPoolBuilder().
			ServerIP(testServerIP1).
			CIDR(testCIDR).
			NetworkName(testNetworkName).
			AgentDeploymentRef(testDeploymentNamespace, testDeploymentName, testImage, "").Build()
		givenNAD := newTestNetworkAttachmentDefinitionBuilder().
			Label(clusterNetworkLabelKey, testClusterNetwork).Build()
		givenDeployment, _ := prepareAgentDeployment(
			NewIPPoolBuilder(testIPPoolNamespace, testIPPoolName).
				ServerIP(testServerIP1).
				CIDR(testCIDR).
				NetworkName(testNetworkName).Build(),
			false,
			testDeploymentNamespace,
			testClusterNetwork,
			testServiceAccountName,
			testImage,
		)

		expectedStatus := newTestIPPoolStatusBuilder().
			AgentDeploymentRef(testDeploymentNamespace, testDeploymentName, testImage, "").Build()
		expectedDeployment, _ := prepareAgentDeployment(
			NewIPPoolBuilder(testIPPoolNamespace, testIPPoolName).
				ServerIP(testServerIP1).
				CIDR(testCIDR).
				NetworkName(testNetworkName).Build(),
			false,
			testDeploymentNamespace,
			testClusterNetwork,
			testServiceAccountName,
			testImage,
		)

		nadGVR := schema.GroupVersionResource{
			Group:    "k8s.cni.cncf.io",
			Version:  "v1",
			Resource: "network-attachment-definitions",
		}

		clientset := fake.NewSimpleClientset()
		err := clientset.Tracker().Create(nadGVR, givenNAD, givenNAD.Namespace)
		assert.Nil(t, err, "mock resource should add into fake controller tracker")

		k8sclientset := k8sfake.NewSimpleClientset()
		err = k8sclientset.Tracker().Add(givenDeployment)
		assert.Nil(t, err, "mock resource should add into fake controller tracker")

		handler := Handler{
			agentNamespace: testDeploymentNamespace,
			agentImage: &config.Image{
				Repository: testImageRepository,
				Tag:        testImageTag,
			},
			agentServiceAccountName: testServiceAccountName,
			nadCache:                fakeclient.NetworkAttachmentDefinitionCache(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions),
			deploymentClient:        fakeclient.DeploymentClient(k8sclientset.AppsV1().Deployments),
			deploymentCache:         fakeclient.DeploymentCache(k8sclientset.AppsV1().Deployments),
		}

		status, err := handler.DeployAgent(givenIPPool, givenIPPool.Status)
		assert.Nil(t, err)
		assert.Equal(t, expectedStatus, status)

		deployment, err := handler.deploymentClient.Get(testDeploymentNamespace, testDeploymentName, metav1.GetOptions{})
		assert.Nil(t, err)

		assert.Equal(t, expectedDeployment, deployment)
	})

	t.Run("very long name ippool created", func(t *testing.T) {
		givenIPPool := NewIPPoolBuilder(testIPPoolNamespace, testIPPoolNameLong).
			ServerIP(testServerIP1).
			CIDR(testCIDR).
			NetworkName(testNetworkNameLong).Build()
		givenNAD := NewNetworkAttachmentDefinitionBuilder(testNADNamespace, testNADNameLong).
			Label(clusterNetworkLabelKey, testClusterNetwork).Build()

		expectedStatus := newTestIPPoolStatusBuilder().
			AgentDeploymentRef(testDeploymentNamespace, testDeploymentNameLong, testImage, "").Build()
		expectedDeployment, _ := prepareAgentDeployment(
			NewIPPoolBuilder(testIPPoolNamespace, testIPPoolNameLong).
				ServerIP(testServerIP1).
				CIDR(testCIDR).
				NetworkName(testNetworkNameLong).Build(),
			false,
			testDeploymentNamespace,
			testClusterNetwork,
			testServiceAccountName,
			testImage,
		)

		nadGVR := schema.GroupVersionResource{
			Group:    "k8s.cni.cncf.io",
			Version:  "v1",
			Resource: "network-attachment-definitions",
		}

		clientset := fake.NewSimpleClientset()
		err := clientset.Tracker().Create(nadGVR, givenNAD, givenNAD.Namespace)
		assert.Nil(t, err, "mock resource should add into fake controller tracker")

		k8sclientset := k8sfake.NewSimpleClientset()

		handler := Handler{
			agentNamespace: testDeploymentNamespace,
			agentImage: &config.Image{
				Repository: testImageRepository,
				Tag:        testImageTag,
			},
			agentServiceAccountName: testServiceAccountName,
			nadCache:                fakeclient.NetworkAttachmentDefinitionCache(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions),
			deploymentClient:        fakeclient.DeploymentClient(k8sclientset.AppsV1().Deployments),
			deploymentCache:         fakeclient.DeploymentCache(k8sclientset.AppsV1().Deployments),
		}

		status, err := handler.DeployAgent(givenIPPool, givenIPPool.Status)
		assert.Nil(t, err)
		assert.Equal(t, expectedStatus, status)

		deployment, err := handler.deploymentClient.Get(testDeploymentNamespace, testDeploymentNameLong, metav1.GetOptions{})
		assert.Nil(t, err)

		assert.Equal(t, expectedDeployment, deployment)
	})

	t.Run("agent deployment upgrade (from main to dev)", func(t *testing.T) {
		givenIPPool := newTestIPPoolBuilder().
			ServerIP(testServerIP1).
			CIDR(testCIDR).
			NetworkName(testNetworkName).
			AgentDeploymentRef(testDeploymentNamespace, testDeploymentName, testImage, "").Build()
		givenNAD := newTestNetworkAttachmentDefinitionBuilder().
			Label(clusterNetworkLabelKey, testClusterNetwork).Build()
		givenDeployment, _ := prepareAgentDeployment(
			NewIPPoolBuilder(testIPPoolNamespace, testIPPoolName).
				ServerIP(testServerIP1).
				CIDR(testCIDR).
				NetworkName(testNetworkName).Build(),
			false,
			testDeploymentNamespace,
			testClusterNetwork,
			testServiceAccountName,
			testImage,
		)

		expectedStatus := newTestIPPoolStatusBuilder().
			AgentDeploymentRef(testDeploymentNamespace, testDeploymentName, testImageNew, "").Build()

		nadGVR := schema.GroupVersionResource{
			Group:    "k8s.cni.cncf.io",
			Version:  "v1",
			Resource: "network-attachment-definitions",
		}

		clientset := fake.NewSimpleClientset()
		err := clientset.Tracker().Create(nadGVR, givenNAD, givenNAD.Namespace)
		assert.Nil(t, err, "mock resource should add into fake controller tracker")

		k8sclientset := k8sfake.NewSimpleClientset()
		err = k8sclientset.Tracker().Add(givenDeployment)
		assert.Nil(t, err, "mock resource should add into fake controller tracker")

		handler := Handler{
			agentNamespace: testDeploymentNamespace,
			agentImage: &config.Image{
				Repository: testImageRepository,
				Tag:        testImageTagNew,
			},
			agentServiceAccountName: testServiceAccountName,
			nadCache:                fakeclient.NetworkAttachmentDefinitionCache(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions),
			deploymentClient:        fakeclient.DeploymentClient(k8sclientset.AppsV1().Deployments),
			deploymentCache:         fakeclient.DeploymentCache(k8sclientset.AppsV1().Deployments),
		}

		status, err := handler.DeployAgent(givenIPPool, givenIPPool.Status)
		assert.Nil(t, err)
		assert.Equal(t, expectedStatus, status)
	})

	t.Run("agent deployment upgrade held back", func(t *testing.T) {
		givenIPPool := newTestIPPoolBuilder().
			Annotation(holdIPPoolAgentUpgradeAnnotationKey, "true").
			ServerIP(testServerIP1).
			CIDR(testCIDR).
			NetworkName(testNetworkName).
			AgentDeploymentRef(testDeploymentNamespace, testDeploymentName, testImage, "").Build()
		givenNAD := newTestNetworkAttachmentDefinitionBuilder().
			Label(clusterNetworkLabelKey, testClusterNetwork).Build()
		givenDeployment, _ := prepareAgentDeployment(
			NewIPPoolBuilder(testIPPoolNamespace, testIPPoolName).
				ServerIP(testServerIP1).
				CIDR(testCIDR).
				NetworkName(testNetworkName).Build(),
			false,
			testDeploymentNamespace,
			testClusterNetwork,
			testServiceAccountName,
			testImage,
		)

		expectedStatus := newTestIPPoolStatusBuilder().
			AgentDeploymentRef(testDeploymentNamespace, testDeploymentName, testImage, "").Build()

		nadGVR := schema.GroupVersionResource{
			Group:    "k8s.cni.cncf.io",
			Version:  "v1",
			Resource: "network-attachment-definitions",
		}

		clientset := fake.NewSimpleClientset()
		err := clientset.Tracker().Create(nadGVR, givenNAD, givenNAD.Namespace)
		assert.Nil(t, err, "mock resource should add into fake controller tracker")

		k8sclientset := k8sfake.NewSimpleClientset()
		err = k8sclientset.Tracker().Add(givenDeployment)
		assert.Nil(t, err, "mock resource should add into fake controller tracker")

		handler := Handler{
			agentNamespace: testDeploymentNamespace,
			agentImage: &config.Image{
				Repository: testImageRepository,
				Tag:        testImageTagNew,
			},
			agentServiceAccountName: testServiceAccountName,
			nadCache:                fakeclient.NetworkAttachmentDefinitionCache(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions),
			deploymentClient:        fakeclient.DeploymentClient(k8sclientset.AppsV1().Deployments),
			deploymentCache:         fakeclient.DeploymentCache(k8sclientset.AppsV1().Deployments),
		}

		status, err := handler.DeployAgent(givenIPPool, givenIPPool.Status)
		assert.Nil(t, err)
		assert.Equal(t, expectedStatus, status)
	})

	t.Run("existing agent deployment uid mismatch", func(t *testing.T) {
		givenIPPool := newTestIPPoolBuilder().
			ServerIP(testServerIP1).
			CIDR(testCIDR).
			NetworkName(testNetworkName).
			AgentDeploymentRef(testDeploymentNamespace, testDeploymentName, testImage, testUID).Build()
		givenNAD := newTestNetworkAttachmentDefinitionBuilder().
			Label(clusterNetworkLabelKey, testClusterNetwork).Build()
		givenDeployment, _ := prepareAgentDeployment(
			NewIPPoolBuilder(testIPPoolNamespace, testIPPoolName).
				ServerIP(testServerIP1).
				CIDR(testCIDR).
				NetworkName(testNetworkName).Build(),
			false,
			testDeploymentNamespace,
			testClusterNetwork,
			testServiceAccountName,
			testImage,
		)

		nadGVR := schema.GroupVersionResource{
			Group:    "k8s.cni.cncf.io",
			Version:  "v1",
			Resource: "network-attachment-definitions",
		}

		clientset := fake.NewSimpleClientset()
		err := clientset.Tracker().Create(nadGVR, givenNAD, givenNAD.Namespace)
		assert.Nil(t, err, "mock resource should add into fake controller tracker")

		k8sclientset := k8sfake.NewSimpleClientset()
		err = k8sclientset.Tracker().Add(givenDeployment)
		assert.Nil(t, err, "mock resource should add into fake controller tracker")

		handler := Handler{
			agentNamespace: testDeploymentNamespace,
			agentImage: &config.Image{
				Repository: testImageRepository,
				Tag:        testImageTagNew,
			},
			agentServiceAccountName: testServiceAccountName,
			nadCache:                fakeclient.NetworkAttachmentDefinitionCache(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions),
			deploymentClient:        fakeclient.DeploymentClient(k8sclientset.AppsV1().Deployments),
			deploymentCache:         fakeclient.DeploymentCache(k8sclientset.AppsV1().Deployments),
		}

		_, err = handler.DeployAgent(givenIPPool, givenIPPool.Status)
		assert.Equal(t, fmt.Sprintf("agent deployment %s uid mismatch", testDeploymentName), err.Error())
	})
}

func TestHandler_BuildCache(t *testing.T) {
	t.Run("new ippool", func(t *testing.T) {
		givenIPAllocator := newTestIPAllocatorBuilder().Build()
		givenCacheAllocator := newTestCacheAllocatorBuilder().Build()
		givenIPPool := newTestIPPoolBuilder().
			CIDR(testCIDR).
			PoolRange(testStartIP, testEndIP).
			NetworkName(testNetworkName).Build()

		expectedIPAllocator := newTestIPAllocatorBuilder().
			IPSubnet(testNetworkName, testCIDR, testStartIP, testEndIP).Build()
		expectedCacheAllocator := newTestCacheAllocatorBuilder().
			MACSet(testNetworkName).Build()

		handler := Handler{
			cacheAllocator: givenCacheAllocator,
			ipAllocator:    givenIPAllocator,
		}

		_, err := handler.BuildCache(givenIPPool, givenIPPool.Status)
		assert.Nil(t, err)

		assert.Equal(t, expectedIPAllocator, handler.ipAllocator)
		assert.Equal(t, expectedCacheAllocator, handler.cacheAllocator)
	})

	t.Run("ippool paused", func(t *testing.T) {
		givenIPPool := newTestIPPoolBuilder().
			Paused().Build()

		handler := Handler{}

		_, err := handler.BuildCache(givenIPPool, givenIPPool.Status)
		assert.Equal(t, fmt.Sprintf("ippool %s was administratively disabled", testIPPoolNamespace+"/"+testIPPoolName), err.Error())
	})

	t.Run("cache is already ready", func(t *testing.T) {
		givenIPPool := newTestIPPoolBuilder().
			CacheReadyCondition(corev1.ConditionTrue, "", "").Build()

		expectedStatus := newTestIPPoolStatusBuilder().
			CacheReadyCondition(corev1.ConditionTrue, "", "").Build()

		handler := Handler{}

		status, err := handler.BuildCache(givenIPPool, givenIPPool.Status)
		assert.Nil(t, err)
		assert.Equal(t, expectedStatus, status)
	})

	t.Run("ippool with excluded ips", func(t *testing.T) {
		givenIPAllocator := newTestIPAllocatorBuilder().Build()
		givenCacheAllocator := newTestCacheAllocatorBuilder().Build()
		givenIPPool := newTestIPPoolBuilder().
			CIDR(testCIDR).
			PoolRange(testStartIP, testEndIP).
			Exclude(testExcludedIP1, testExcludedIP2).
			NetworkName(testNetworkName).Build()

		expectedIPAllocator := newTestIPAllocatorBuilder().
			IPSubnet(testNetworkName, testCIDR, testStartIP, testEndIP).
			Revoke(testNetworkName, testExcludedIP1, testExcludedIP2).Build()
		expectedCacheAllocator := newTestCacheAllocatorBuilder().
			MACSet(testNetworkName).Build()

		handler := Handler{
			cacheAllocator: givenCacheAllocator,
			ipAllocator:    givenIPAllocator,
		}

		_, err := handler.BuildCache(givenIPPool, givenIPPool.Status)
		assert.Nil(t, err)

		assert.Equal(t, expectedIPAllocator, handler.ipAllocator)
		assert.Equal(t, expectedCacheAllocator, handler.cacheAllocator)
	})

	t.Run("rebuild caches", func(t *testing.T) {
		givenIPAllocator := newTestIPAllocatorBuilder().Build()
		givenCacheAllocator := newTestCacheAllocatorBuilder().Build()
		givenIPPool := newTestIPPoolBuilder().
			CIDR(testCIDR).
			PoolRange(testStartIP, testEndIP).
			Exclude(testExcludedIP1, testExcludedIP2).
			NetworkName(testNetworkName).
			Allocated(testAllocatedIP1, testMAC1).
			Allocated(testAllocatedIP2, testMAC2).Build()

		expectedIPAllocator := newTestIPAllocatorBuilder().
			IPSubnet(testNetworkName, testCIDR, testStartIP, testEndIP).
			Revoke(testNetworkName, testExcludedIP1, testExcludedIP2).
			Allocate(testNetworkName, testAllocatedIP1, testAllocatedIP2).Build()
		expectedCacheAllocator := newTestCacheAllocatorBuilder().
			MACSet(testNetworkName).
			Add(testNetworkName, testMAC1, testAllocatedIP1).
			Add(testNetworkName, testMAC2, testAllocatedIP2).Build()

		handler := Handler{
			cacheAllocator: givenCacheAllocator,
			ipAllocator:    givenIPAllocator,
		}

		_, err := handler.BuildCache(givenIPPool, givenIPPool.Status)
		assert.Nil(t, err)

		assert.Equal(t, expectedIPAllocator, handler.ipAllocator)
		assert.Equal(t, expectedCacheAllocator, handler.cacheAllocator)
	})
}

func TestHandler_MonitorAgent(t *testing.T) {
	t.Run("agent deployment not found", func(t *testing.T) {
		givenIPPool := newTestIPPoolBuilder().AgentDeploymentRef(testDeploymentNamespace, testDeploymentName, testImage, "").Build()
		givenDeployment := newDeploymentBuilder("default", "nginx").Build()

		k8sclientset := k8sfake.NewSimpleClientset()

		err := k8sclientset.Tracker().Add(givenDeployment)
		assert.Nil(t, err, "mock resource should add into fake controller tracker")

		handler := Handler{
			deploymentCache: fakeclient.DeploymentCache(k8sclientset.AppsV1().Deployments),
		}

		_, err = handler.MonitorAgent(givenIPPool, givenIPPool.Status)
		assert.Equal(t, fmt.Sprintf("deployments.apps \"%s\" not found", testDeploymentName), err.Error())
	})

	t.Run("agent deployment unready", func(t *testing.T) {
		givenIPPool := newTestIPPoolBuilder().AgentDeploymentRef(testDeploymentNamespace, testDeploymentName, testImage, "").Build()
		givenDeployment := newTestDeploymentBuilder().
			Container(testContainerName, testImageRepository, testImageTag).Build()

		k8sclientset := k8sfake.NewSimpleClientset()

		err := k8sclientset.Tracker().Add(givenDeployment)
		assert.Nil(t, err, "mock resource should add into fake controller tracker")

		handler := Handler{
			deploymentCache: fakeclient.DeploymentCache(k8sclientset.AppsV1().Deployments),
		}

		_, err = handler.MonitorAgent(givenIPPool, givenIPPool.Status)
		assert.Equal(t, fmt.Sprintf("agent deployment %s not ready", testDeploymentName), err.Error())
	})

	t.Run("agent deployment ready", func(t *testing.T) {
		givenIPPool := newTestIPPoolBuilder().AgentDeploymentRef(testDeploymentNamespace, testDeploymentName, testImage, "").Build()
		givenDeployment := newTestDeploymentBuilder().
			Container(testContainerName, testImageRepository, testImageTag).
			DeploymentReady(true).Build()

		k8sclientset := k8sfake.NewSimpleClientset()

		err := k8sclientset.Tracker().Add(givenDeployment)
		assert.Nil(t, err, "mock resource should add into fake controller tracker")

		handler := Handler{
			deploymentCache: fakeclient.DeploymentCache(k8sclientset.AppsV1().Deployments),
		}

		_, err = handler.MonitorAgent(givenIPPool, givenIPPool.Status)
		assert.Nil(t, err)
	})

	t.Run("ippool paused", func(t *testing.T) {
		givenIPPool := newTestIPPoolBuilder().Paused().Build()

		handler := Handler{}

		_, err := handler.MonitorAgent(givenIPPool, givenIPPool.Status)
		assert.Equal(t, fmt.Sprintf("ippool %s was administratively disabled", testIPPoolNamespace+"/"+testIPPoolName), err.Error())
	})

	t.Run("ippool in no-agent mode", func(t *testing.T) {
		givenIPPool := newTestIPPoolBuilder().Build()

		handler := Handler{
			noAgent: true,
		}

		_, err := handler.MonitorAgent(givenIPPool, givenIPPool.Status)
		assert.Nil(t, err)
	})

	t.Run("agent deployment ref not set", func(t *testing.T) {
		givenIPPool := newTestIPPoolBuilder().Build()

		handler := Handler{}

		_, err := handler.MonitorAgent(givenIPPool, givenIPPool.Status)
		assert.Equal(t, fmt.Sprintf("agent for ippool %s is not deployed", testIPPoolNamespace+"/"+testIPPoolName), err.Error())
	})

	t.Run("outdated agent deployment", func(t *testing.T) {
		givenIPPool := newTestIPPoolBuilder().
			AgentDeploymentRef(testDeploymentNamespace, testDeploymentName, testImageNew, "").Build()
		givenDeployment := newTestDeploymentBuilder().
			Container(testContainerName, testImageRepository, testImageTag).
			DeploymentReady(true).Build()

		k8sclientset := k8sfake.NewSimpleClientset()

		err := k8sclientset.Tracker().Add(givenDeployment)
		assert.Nil(t, err, "mock resource should add into fake controller tracker")

		handler := Handler{
			deploymentClient: fakeclient.DeploymentClient(k8sclientset.AppsV1().Deployments),
			deploymentCache:  fakeclient.DeploymentCache(k8sclientset.AppsV1().Deployments),
		}

		_, err = handler.MonitorAgent(givenIPPool, givenIPPool.Status)
		assert.Equal(t, fmt.Sprintf("agent deployment %s image mismatch", testDeploymentName), err.Error())

	})
}
