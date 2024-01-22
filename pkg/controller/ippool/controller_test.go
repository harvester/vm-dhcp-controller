package ippool

import (
	"testing"

	cniv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/rancher/wrangler/pkg/genericcondition"
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
	"github.com/harvester/vm-dhcp-controller/pkg/util/fakeclient"
)

const (
	testIPPoolNamespace    = "default"
	testIPPoolName         = "ippool-1"
	testPodNamespace       = "default"
	testPodName            = "default-ippool-1-agent"
	testNADNamespace       = "default"
	testNADName            = "net-1"
	testClusterNetworkName = "provider"
)

func newTestIPPoolBuilder() *ipPoolBuilder {
	return newIPPoolBuilder(testIPPoolNamespace, testIPPoolName)
}

func newTestPodBuilder() *podBuilder {
	return newPodBuilder(testPodNamespace, testPodName)
}

func newTestNetworkAttachmentDefinitionBuilder() *networkAttachmentDefinitionBuilder {
	return newNetworkAttachmentDefinitionBuilder(testNADNamespace, testNADName)
}

func TestHandler_OnChange(t *testing.T) {
	type input struct {
		key    string
		ipPool *networkv1.IPPool
		pods   []*corev1.Pod
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
			name: "pause ippool",
			given: input{
				key: "default/ippool-1",
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
					DisabledCondition(corev1.ConditionTrue, "", "").
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
			ipAllocator:      ipam.New(),
			metricsAllocator: metrics.New(),
			ippoolClient:     fakeclient.IPPoolClient(clientset.NetworkV1alpha1().IPPools),
			podClient:        fakeclient.PodClient(k8sclientset.CoreV1().Pods),
		}

		var actual output

		actual.ipPool, actual.err = handler.OnChange(tc.given.key, tc.given.ipPool)
		assert.Nil(t, actual.err)

		emptyConditionsTimestamp(tc.expected.ipPool.Status.Conditions)
		emptyConditionsTimestamp(actual.ipPool.Status.Conditions)
		assert.Equal(t, tc.expected.ipPool, actual.ipPool, tc.name)

		assert.Equal(t, tc.expected.pods, actual.pods)
	}
}

func TestHandler_DeployAgent(t *testing.T) {
	type input struct {
		key    string
		ipPool *networkv1.IPPool
		nad    *cniv1.NetworkAttachmentDefinition
	}

	type output struct {
		ipPoolStatus networkv1.IPPoolStatus
		pod          *corev1.Pod
		err          error
	}

	testCases := []struct {
		name     string
		given    input
		expected output
	}{
		{
			name: "resume ippool",
			given: input{
				key: "default/ippool-1",
				ipPool: newTestIPPoolBuilder().
					ServerIP("192.168.0.2").
					CIDR("192.168.0.0/24").
					NetworkName("default/net-1").
					Build(),
				nad: newTestNetworkAttachmentDefinitionBuilder().
					Label(clusterNetworkLabelKey, testClusterNetworkName).
					Build(),
			},
			expected: output{
				ipPoolStatus: networkv1.IPPoolStatus{
					AgentPodRef: &networkv1.PodReference{
						Namespace: "default",
						Name:      "default-ippool-1-agent",
					},
				},
				pod: prepareAgentPod(
					newIPPoolBuilder("default", "ippool-1").
						ServerIP("192.168.0.2").
						CIDR("192.168.0.0/24").
						NetworkName("default/net-1").
						Build(),
					false,
					"default",
					"provider",
					"vdca",
					&config.Image{
						Repository: "rancher/harvester-vm-dhcp-controller",
						Tag:        "main",
					},
				),
			},
		},
	}

	nadGVR := schema.GroupVersionResource{
		Group:    "k8s.cni.cncf.io",
		Version:  "v1",
		Resource: "network-attachment-definitions",
	}

	for _, tc := range testCases {
		clientset := fake.NewSimpleClientset(tc.given.ipPool)
		if tc.given.nad != nil {
			err := clientset.Tracker().Create(nadGVR, tc.given.nad, tc.given.nad.Namespace)
			assert.Nil(t, err, "mock resource should add into fake controller tracker")
		}

		k8sclientset := k8sfake.NewSimpleClientset()

		handler := Handler{
			agentNamespace: "default",
			agentImage: &config.Image{
				Repository: "rancher/harvester-vm-dhcp-controller",
				Tag:        "main",
			},
			agentServiceAccountName: "vdca",
			cacheAllocator:          cache.New(),
			ipAllocator:             ipam.New(),
			metricsAllocator:        metrics.New(),
			ippoolClient:            fakeclient.IPPoolClient(clientset.NetworkV1alpha1().IPPools),
			nadCache:                fakeclient.NetworkAttachmentDefinitionCache(clientset.K8sCniCncfIoV1().NetworkAttachmentDefinitions),
			podClient:               fakeclient.PodClient(k8sclientset.CoreV1().Pods),
		}

		var actual output

		actual.ipPoolStatus, actual.err = handler.DeployAgent(tc.given.ipPool, tc.given.ipPool.Status)
		assert.Nil(t, actual.err)

		emptyConditionsTimestamp(tc.expected.ipPoolStatus.Conditions)
		emptyConditionsTimestamp(actual.ipPoolStatus.Conditions)
		assert.Equal(t, tc.expected.ipPoolStatus, actual.ipPoolStatus, tc.name)

		actual.pod, actual.err = handler.podClient.Get("default", "default-ippool-1-agent", metav1.GetOptions{})
		assert.Nil(t, actual.err)
		assert.Equal(t, tc.expected.pod, actual.pod)
	}
}

func emptyConditionsTimestamp(conditions []genericcondition.GenericCondition) {
	for i := range conditions {
		conditions[i].LastTransitionTime = ""
		conditions[i].LastUpdateTime = ""
	}
}
