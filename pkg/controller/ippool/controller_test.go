package ippool

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	networkv1 "github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/harvester/vm-dhcp-controller/pkg/cache"
	"github.com/harvester/vm-dhcp-controller/pkg/config"
	"github.com/harvester/vm-dhcp-controller/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/vm-dhcp-controller/pkg/ipam"
	"github.com/harvester/vm-dhcp-controller/pkg/metrics"
	"github.com/harvester/vm-dhcp-controller/pkg/util"
	"github.com/harvester/vm-dhcp-controller/pkg/util/fakeclient"
)

const (
	testNADNamespace       = "default"
	testNADName            = "net-1"
	testNADNameLong        = "fi6cx9ca1kt1faq80k3ro9cowyumyjb67qdmg8fb9ydmz27rbk5btlg2m5avv3n"
	testIPPoolNamespace    = testNADNamespace
	testIPPoolName         = testNADName
	testIPPoolNameLong     = testNADNameLong
	testKey                = testIPPoolNamespace + "/" + testIPPoolName
	testPodNamespace       = "harvester-system"
	testPodName            = testNADNamespace + "-" + testNADName + "-agent"
	testClusterNetwork     = "provider"
	testServerIP           = "192.168.0.2"
	testNetworkName        = testNADNamespace + "/" + testNADName
	testNetworkNameLong    = testNADNamespace + "/" + testNADNameLong
	testCIDR               = "192.168.0.0/24"
	testStartIP            = "192.168.0.101"
	testEndIP              = "192.168.0.200"
	testServiceAccountName = "vdca"
	testImageRepository    = "rancher/harvester-vm-dhcp-controller"
	testImageTag           = "main"

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
	testPodNameLong = util.SafeAgentConcatName(testNADNamespace, testNADNameLong)
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

func newTestPodBuilder() *podBuilder {
	return newPodBuilder(testPodNamespace, testPodName)
}

func newTestIPPoolStatusBuilder() *ipPoolStatusBuilder {
	return newIPPoolStatusBuilder()
}

func newTestNetworkAttachmentDefinitionBuilder() *networkAttachmentDefinitionBuilder {
	return newNetworkAttachmentDefinitionBuilder(testNADNamespace, testNADName)
}

func TestHandler_OnChange(t *testing.T) {
	type input struct {
		key         string
		ipAllocator *ipam.IPAllocator
		ipPool      *networkv1.IPPool
		pods        []*corev1.Pod
	}

	type output struct {
		ipPool *networkv1.IPPool
		pods   []*corev1.Pod
		err    error
	}

	testCases := []struct {
		name     string
		given    input
		expected output
	}{
		{
			name: "new ippool",
			given: input{
				key: testIPPoolNamespace + "/" + testIPPoolName,
				ipAllocator: newTestIPAllocatorBuilder().
					Build(),
				ipPool: newTestIPPoolBuilder().
					Build(),
			},
			expected: output{
				ipPool: newTestIPPoolBuilder().
					StoppedCondition(corev1.ConditionFalse, "", "").
					CacheReadyCondition(corev1.ConditionFalse, "NotInitialized", "").
					Build(),
			},
		},
		{
			name: "ippool with ipam initialized",
			given: input{
				key: testIPPoolNamespace + "/" + testIPPoolName,
				ipAllocator: newTestIPAllocatorBuilder().
					IPSubnet(testNetworkName, testCIDR, testStartIP, testEndIP).
					Build(),
				ipPool: newTestIPPoolBuilder().
					ServerIP(testServerIP).
					CIDR(testCIDR).
					PoolRange(testStartIP, testEndIP).
					NetworkName(testNetworkName).
					Build(),
			},
			expected: output{
				ipPool: newTestIPPoolBuilder().
					ServerIP(testServerIP).
					CIDR(testCIDR).
					PoolRange(testStartIP, testEndIP).
					NetworkName(testNetworkName).
					Available(100).
					Used(0).
					StoppedCondition(corev1.ConditionFalse, "", "").
					Build(),
			},
		},
		{
			name: "pause ippool",
			given: input{
				key: testIPPoolNamespace + "/" + testIPPoolName,
				ipAllocator: newTestIPAllocatorBuilder().
					Build(),
				ipPool: newTestIPPoolBuilder().
					Paused().
					AgentPodRef("default", "default-ippool-1-agent").
					Build(),
				pods: []*corev1.Pod{
					newTestPodBuilder().
						Build(),
				},
			},
			expected: output{
				ipPool: newTestIPPoolBuilder().
					Paused().
					StoppedCondition(corev1.ConditionTrue, "", "").
					Build(),
			},
		},
		{
			name: "resume ippool",
			given: input{
				key: testIPPoolNamespace + "/" + testIPPoolName,
				ipAllocator: newTestIPAllocatorBuilder().
					Build(),
				ipPool: newTestIPPoolBuilder().
					UnPaused().
					Build(),
			},
			expected: output{
				ipPool: newTestIPPoolBuilder().
					UnPaused().
					StoppedCondition(corev1.ConditionFalse, "", "").
					CacheReadyCondition(corev1.ConditionFalse, "NotInitialized", "").
					Build(),
			},
		},
	}

	for _, tc := range testCases {
		clientset := fake.NewSimpleClientset()
		err := clientset.Tracker().Add(tc.given.ipPool)
		if err != nil {
			t.Fatal(err)
		}

		var pods []runtime.Object
		for _, pod := range tc.given.pods {
			pods = append(pods, pod)
		}
		k8sclientset := k8sfake.NewSimpleClientset(pods...)
		handler := Handler{
			agentNamespace: "default",
			agentImage: &config.Image{
				Repository: "rancher/harvester-vm-dhcp-controller",
				Tag:        "main",
			},
			cacheAllocator:   cache.New(),
			ipAllocator:      tc.given.ipAllocator,
			metricsAllocator: metrics.New(),
			ippoolClient:     fakeclient.IPPoolClient(clientset.NetworkV1alpha1().IPPools),
			podClient:        fakeclient.PodClient(k8sclientset.CoreV1().Pods),
		}

		var actual output

		actual.ipPool, actual.err = handler.OnChange(tc.given.key, tc.given.ipPool)
		assert.Nil(t, actual.err)

		SanitizeStatus(&tc.expected.ipPool.Status)
		SanitizeStatus(&actual.ipPool.Status)

		assert.Equal(t, tc.expected.ipPool, actual.ipPool, tc.name)

		assert.Equal(t, tc.expected.pods, actual.pods)
	}
}

func TestHandler_DeployAgent(t *testing.T) {
	t.Run("ippool created", func(t *testing.T) {
		givenIPPool := newTestIPPoolBuilder().
			ServerIP(testServerIP).
			CIDR(testCIDR).
			NetworkName(testNetworkName).Build()
		givenNAD := newTestNetworkAttachmentDefinitionBuilder().
			Label(clusterNetworkLabelKey, testClusterNetwork).Build()

		expectedStatus := newTestIPPoolStatusBuilder().
			AgentPodRef(testPodNamespace, testPodName).Build()
		expectedPod := prepareAgentPod(
			NewIPPoolBuilder(testIPPoolNamespace, testIPPoolName).
				ServerIP(testServerIP).
				CIDR(testCIDR).
				NetworkName(testNetworkName).Build(),
			false,
			testPodNamespace,
			testClusterNetwork,
			testServiceAccountName,
			&config.Image{
				Repository: testImageRepository,
				Tag:        testImageTag,
			},
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
			agentNamespace: testPodNamespace,
			agentImage: &config.Image{
				Repository: testImageRepository,
				Tag:        testImageTag,
			},
			agentServiceAccountName: testServiceAccountName,
			nadCache:                fakeclient.NetworkAttachmentDefinitionCache(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions),
			podClient:               fakeclient.PodClient(k8sclientset.CoreV1().Pods),
		}

		status, err := handler.DeployAgent(givenIPPool, givenIPPool.Status)
		assert.Nil(t, err)
		assert.Equal(t, expectedStatus, status)

		pod, err := handler.podClient.Get(testPodNamespace, testPodName, metav1.GetOptions{})
		assert.Nil(t, err)
		assert.Equal(t, expectedPod, pod)
	})

	t.Run("ippool paused", func(t *testing.T) {
		givenIPPool := newTestIPPoolBuilder().
			Paused().Build()

		handler := Handler{
			agentNamespace: testPodNamespace,
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

	t.Run("agent pod already exists", func(t *testing.T) {
		givenIPPool := newTestIPPoolBuilder().
			ServerIP(testServerIP).
			CIDR(testCIDR).
			NetworkName(testNetworkName).
			AgentPodRef(testPodNamespace, testPodName).Build()
		givenNAD := newTestNetworkAttachmentDefinitionBuilder().
			Label(clusterNetworkLabelKey, testClusterNetwork).Build()
		givenPod := prepareAgentPod(
			NewIPPoolBuilder(testIPPoolNamespace, testIPPoolName).
				ServerIP(testServerIP).
				CIDR(testCIDR).
				NetworkName(testNetworkName).Build(),
			false,
			testPodNamespace,
			testClusterNetwork,
			testServiceAccountName,
			&config.Image{
				Repository: testImageRepository,
				Tag:        testImageTag,
			},
		)

		expectedStatus := newTestIPPoolStatusBuilder().
			AgentPodRef(testPodNamespace, testPodName).Build()
		expectedPod := prepareAgentPod(
			NewIPPoolBuilder(testIPPoolNamespace, testIPPoolName).
				ServerIP(testServerIP).
				CIDR(testCIDR).
				NetworkName(testNetworkName).Build(),
			false,
			testPodNamespace,
			testClusterNetwork,
			testServiceAccountName,
			&config.Image{
				Repository: testImageRepository,
				Tag:        testImageTag,
			},
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
		err = k8sclientset.Tracker().Add(givenPod)
		assert.Nil(t, err, "mock resource should add into fake controller tracker")

		handler := Handler{
			agentNamespace: testPodNamespace,
			agentImage: &config.Image{
				Repository: testImageRepository,
				Tag:        testImageTag,
			},
			agentServiceAccountName: testServiceAccountName,
			nadCache:                fakeclient.NetworkAttachmentDefinitionCache(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions),
			podClient:               fakeclient.PodClient(k8sclientset.CoreV1().Pods),
		}

		status, err := handler.DeployAgent(givenIPPool, givenIPPool.Status)
		assert.Nil(t, err)
		assert.Equal(t, expectedStatus, status)

		pod, err := handler.podClient.Get(testPodNamespace, testPodName, metav1.GetOptions{})
		assert.Nil(t, err)
		assert.Equal(t, expectedPod, pod)
	})

	t.Run("very long name ippool created", func(t *testing.T) {
		givenIPPool := NewIPPoolBuilder(testIPPoolNamespace, testIPPoolNameLong).
			ServerIP(testServerIP).
			CIDR(testCIDR).
			NetworkName(testNetworkNameLong).Build()
		givenNAD := newNetworkAttachmentDefinitionBuilder(testNADNamespace, testNADNameLong).
			Label(clusterNetworkLabelKey, testClusterNetwork).Build()

		expectedStatus := newTestIPPoolStatusBuilder().
			AgentPodRef(testPodNamespace, testPodNameLong).Build()
		expectedPod := prepareAgentPod(
			NewIPPoolBuilder(testIPPoolNamespace, testIPPoolNameLong).
				ServerIP(testServerIP).
				CIDR(testCIDR).
				NetworkName(testNetworkNameLong).Build(),
			false,
			testPodNamespace,
			testClusterNetwork,
			testServiceAccountName,
			&config.Image{
				Repository: testImageRepository,
				Tag:        testImageTag,
			},
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
			agentNamespace: testPodNamespace,
			agentImage: &config.Image{
				Repository: testImageRepository,
				Tag:        testImageTag,
			},
			agentServiceAccountName: testServiceAccountName,
			nadCache:                fakeclient.NetworkAttachmentDefinitionCache(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions),
			podClient:               fakeclient.PodClient(k8sclientset.CoreV1().Pods),
		}

		status, err := handler.DeployAgent(givenIPPool, givenIPPool.Status)
		assert.Nil(t, err)
		assert.Equal(t, expectedStatus, status)

		pod, err := handler.podClient.Get(testPodNamespace, testPodNameLong, metav1.GetOptions{})
		assert.Nil(t, err)
		assert.Equal(t, expectedPod, pod)
	})
}

func TestHandler_BuildCache(t *testing.T) {
	t.Run("new ippool", func(t *testing.T) {
		givenIPAllocator := newTestIPAllocatorBuilder().
			Build()
		givenCacheAllocator := newTestCacheAllocatorBuilder().
			Build()
		givenIPPool := newTestIPPoolBuilder().
			CIDR(testCIDR).
			PoolRange(testStartIP, testEndIP).
			NetworkName(testNetworkName).
			Build()

		expectedIPAllocator := newTestIPAllocatorBuilder().
			IPSubnet(testNetworkName, testCIDR, testStartIP, testEndIP).
			Build()
		expectedCacheAllocator := newTestCacheAllocatorBuilder().
			MACSet(testNetworkName).
			Build()

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
			Paused().
			Build()

		handler := Handler{}

		_, err := handler.BuildCache(givenIPPool, givenIPPool.Status)
		assert.Equal(t, fmt.Sprintf("ippool %s was administratively disabled", testIPPoolNamespace+"/"+testIPPoolName), err.Error())
	})

	t.Run("cache is already ready", func(t *testing.T) {
		givenIPPool := newTestIPPoolBuilder().
			CacheReadyCondition(corev1.ConditionTrue, "", "").
			Build()

		expectedStatus := newTestIPPoolStatusBuilder().
			CacheReadyCondition(corev1.ConditionTrue, "", "").
			Build()

		handler := Handler{}

		status, err := handler.BuildCache(givenIPPool, givenIPPool.Status)
		assert.Nil(t, err)
		assert.Equal(t, expectedStatus, status)
	})

	t.Run("ippool with excluded ips", func(t *testing.T) {
		givenIPAllocator := newTestIPAllocatorBuilder().
			Build()
		givenCacheAllocator := newTestCacheAllocatorBuilder().
			Build()
		givenIPPool := newTestIPPoolBuilder().
			CIDR(testCIDR).
			PoolRange(testStartIP, testEndIP).
			Exclude(testExcludedIP1, testExcludedIP2).
			NetworkName(testNetworkName).
			Build()

		expectedIPAllocator := newTestIPAllocatorBuilder().
			IPSubnet(testNetworkName, testCIDR, testStartIP, testEndIP).
			Revoke(testNetworkName, testExcludedIP1, testExcludedIP2).
			Build()
		expectedCacheAllocator := newTestCacheAllocatorBuilder().
			MACSet(testNetworkName).
			Build()

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
		givenIPAllocator := newTestIPAllocatorBuilder().
			Build()
		givenCacheAllocator := newTestCacheAllocatorBuilder().
			Build()
		givenIPPool := newTestIPPoolBuilder().
			CIDR(testCIDR).
			PoolRange(testStartIP, testEndIP).
			Exclude(testExcludedIP1, testExcludedIP2).
			NetworkName(testNetworkName).
			Allocated(testAllocatedIP1, testMAC1).
			Allocated(testAllocatedIP2, testMAC2).
			Build()

		expectedIPAllocator := newTestIPAllocatorBuilder().
			IPSubnet(testNetworkName, testCIDR, testStartIP, testEndIP).
			Revoke(testNetworkName, testExcludedIP1, testExcludedIP2).
			Allocate(testNetworkName, testAllocatedIP1, testAllocatedIP2).
			Build()
		expectedCacheAllocator := newTestCacheAllocatorBuilder().
			MACSet(testNetworkName).
			Add(testNetworkName, testMAC1, testAllocatedIP1).
			Add(testNetworkName, testMAC2, testAllocatedIP2).
			Build()

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
	t.Run("agent pod not found", func(t *testing.T) {
		givenIPPool := newTestIPPoolBuilder().AgentPodRef(testPodNamespace, testPodName).Build()
		givenPod := newPodBuilder("default", "nginx").Build()

		k8sclientset := k8sfake.NewSimpleClientset()

		err := k8sclientset.Tracker().Add(givenPod)
		assert.Nil(t, err, "mock resource should add into fake controller tracker")

		handler := Handler{
			podCache: fakeclient.PodCache(k8sclientset.CoreV1().Pods),
		}

		_, err = handler.MonitorAgent(givenIPPool, givenIPPool.Status)
		assert.Equal(t, fmt.Sprintf("pods \"%s\" not found", testPodName), err.Error())
	})

	t.Run("agent pod unready", func(t *testing.T) {
		givenIPPool := newTestIPPoolBuilder().AgentPodRef(testPodNamespace, testPodName).Build()
		givenPod := newTestPodBuilder().Build()

		k8sclientset := k8sfake.NewSimpleClientset()

		err := k8sclientset.Tracker().Add(givenPod)
		assert.Nil(t, err, "mock resource should add into fake controller tracker")

		handler := Handler{
			podCache: fakeclient.PodCache(k8sclientset.CoreV1().Pods),
		}

		_, err = handler.MonitorAgent(givenIPPool, givenIPPool.Status)
		assert.Equal(t, fmt.Sprintf("agent for ippool %s is not ready", testPodNamespace+"/"+testPodName), err.Error())
	})

	t.Run("agent pod ready", func(t *testing.T) {
		givenIPPool := newTestIPPoolBuilder().AgentPodRef(testPodNamespace, testPodName).Build()
		givenPod := newTestPodBuilder().PodReady(corev1.ConditionTrue).Build()

		k8sclientset := k8sfake.NewSimpleClientset()

		err := k8sclientset.Tracker().Add(givenPod)
		assert.Nil(t, err, "mock resource should add into fake controller tracker")

		handler := Handler{
			podCache: fakeclient.PodCache(k8sclientset.CoreV1().Pods),
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

	t.Run("agentpodref not set", func(t *testing.T) {
		givenIPPool := newTestIPPoolBuilder().Build()

		handler := Handler{}

		_, err := handler.MonitorAgent(givenIPPool, givenIPPool.Status)
		assert.Equal(t, fmt.Sprintf("agent for ippool %s is not deployed", testIPPoolNamespace+"/"+testIPPoolName), err.Error())
	})
}
