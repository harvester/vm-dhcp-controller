package vm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	networkv1 "github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/harvester/vm-dhcp-controller/pkg/controller/vmnetcfg"
	"github.com/harvester/vm-dhcp-controller/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/vm-dhcp-controller/pkg/util/fakeclient"
	"github.com/harvester/vm-dhcp-controller/pkg/util/fakecontroller"
)

const (
	testVMNamespace       = "default"
	testVMName            = "test-vm"
	testKey               = testVMNamespace + "/" + testVMName
	testNADNamespace      = "default"
	testNADName           = "test-nad"
	testNetworkName       = testNADNamespace + "/" + testNADName
	testMACAddress1       = "11:22:33:44:55:66"
	testMACAddress2       = "22:33:44:55:66:77"
	testIPAddress         = "192.168.100.100"
	testNICName           = "nic1"
	testVmNetCfgNamespace = "default"
	testVmNetCfgName      = "test-vm"
)

func newTestVMBuilder() *vmBuilder {
	return newVMBuilder(testVMNamespace, testVMName)
}

func newTestVmNetCfgBuilder() *vmnetcfg.VmNetCfgBuilder {
	return vmnetcfg.NewVmNetCfgBuilder(testVmNetCfgNamespace, testVmNetCfgName)
}

func TestHandler_OnChange(t *testing.T) {
	t.Run("new vm without mac", func(t *testing.T) {
		givenVM := newTestVMBuilder().
			WithInterfaceInAnnotation("", testNICName).
			WithInterfaceInSpec("", testNICName).
			WithNetwork(testNICName, testNetworkName).Build()

		clientset := fake.NewSimpleClientset()
		err := clientset.Tracker().Add(givenVM)
		if err != nil {
			t.Fatal(err)
		}

		handler := Handler{
			vmnetcfgCache:  fakeclient.VirtualMachineNetworkConfigCache(clientset.NetworkV1alpha1().VirtualMachineNetworkConfigs),
			vmnetcfgClient: fakeclient.VirtualMachineNetworkConfigClient(clientset.NetworkV1alpha1().VirtualMachineNetworkConfigs),
		}

		_, err = handler.OnChange(testKey, givenVM)
		assert.Nil(t, err)

		_, err = handler.vmnetcfgClient.Get(testVmNetCfgNamespace, testVmNetCfgName, metav1.GetOptions{})
		assert.NotNil(t, err, "expected error when getting vmnetcfg")
	})

	t.Run("new vm with mac in annotation", func(t *testing.T) {
		givenVM := newTestVMBuilder().
			WithInterfaceInAnnotation(testMACAddress1, testNICName).
			WithNetwork(testNICName, testNetworkName).Build()

		expectedVmNetCfg := newTestVmNetCfgBuilder().
			Label(vmLabelKey, testVMName).
			OwnerRef(metav1.OwnerReference{
				Name: testVMName,
			}).
			WithVMName(testVMName).
			WithNetworkConfig("", testMACAddress1, testNetworkName).Build()

		clientset := fake.NewSimpleClientset()
		err := clientset.Tracker().Add(givenVM)
		if err != nil {
			t.Fatal(err)
		}

		handler := Handler{
			vmnetcfgCache:  fakeclient.VirtualMachineNetworkConfigCache(clientset.NetworkV1alpha1().VirtualMachineNetworkConfigs),
			vmnetcfgClient: fakeclient.VirtualMachineNetworkConfigClient(clientset.NetworkV1alpha1().VirtualMachineNetworkConfigs),
		}

		_, err = handler.OnChange(testKey, givenVM)
		assert.Nil(t, err)

		vmNetCfg, err := handler.vmnetcfgClient.Get(testVmNetCfgNamespace, testVmNetCfgName, metav1.GetOptions{})
		assert.Nil(t, err)
		assert.Equal(t, expectedVmNetCfg, vmNetCfg)
	})

	t.Run("new vm with mac in spec", func(t *testing.T) {
		givenVM := newTestVMBuilder().
			WithInterfaceInSpec(testMACAddress1, testNICName).
			WithNetwork(testNICName, testNetworkName).Build()

		expectedVmNetCfg := newTestVmNetCfgBuilder().
			Label(vmLabelKey, testVMName).
			OwnerRef(metav1.OwnerReference{
				Name: testVMName,
			}).
			WithVMName(testVMName).
			WithNetworkConfig("", testMACAddress1, testNetworkName).Build()

		clientset := fake.NewSimpleClientset()
		err := clientset.Tracker().Add(givenVM)
		if err != nil {
			t.Fatal(err)
		}

		handler := Handler{
			vmnetcfgCache:  fakeclient.VirtualMachineNetworkConfigCache(clientset.NetworkV1alpha1().VirtualMachineNetworkConfigs),
			vmnetcfgClient: fakeclient.VirtualMachineNetworkConfigClient(clientset.NetworkV1alpha1().VirtualMachineNetworkConfigs),
		}

		_, err = handler.OnChange(testKey, givenVM)
		assert.Nil(t, err)

		vmNetCfg, err := handler.vmnetcfgClient.Get(testVmNetCfgNamespace, testVmNetCfgName, metav1.GetOptions{})
		assert.Nil(t, err)
		assert.Equal(t, expectedVmNetCfg, vmNetCfg)
	})

	t.Run("new vm with different mac address in both annotation and spec", func(t *testing.T) {
		givenVM := newTestVMBuilder().
			WithInterfaceInAnnotation(testMACAddress1, testNICName).
			WithInterfaceInSpec(testMACAddress2, testNICName).
			WithNetwork(testNICName, testNetworkName).Build()

		expectedVmNetCfg := newTestVmNetCfgBuilder().
			Label(vmLabelKey, testVMName).
			OwnerRef(metav1.OwnerReference{
				Name: testVMName,
			}).
			WithVMName(testVMName).
			WithNetworkConfig("", testMACAddress2, testNetworkName).Build()

		clientset := fake.NewSimpleClientset()
		err := clientset.Tracker().Add(givenVM)
		if err != nil {
			t.Fatal(err)
		}

		handler := Handler{
			vmnetcfgCache:  fakeclient.VirtualMachineNetworkConfigCache(clientset.NetworkV1alpha1().VirtualMachineNetworkConfigs),
			vmnetcfgClient: fakeclient.VirtualMachineNetworkConfigClient(clientset.NetworkV1alpha1().VirtualMachineNetworkConfigs),
		}

		_, err = handler.OnChange(testKey, givenVM)
		assert.Nil(t, err)

		vmNetCfg, err := handler.vmnetcfgClient.Get(testVmNetCfgNamespace, testVmNetCfgName, metav1.GetOptions{})
		assert.Nil(t, err)
		assert.Equal(t, expectedVmNetCfg, vmNetCfg)
	})

	t.Run("new vm attaching to pod network", func(t *testing.T) {
		givenVM := newTestVMBuilder().
			WithInterfaceInAnnotation(testMACAddress1, testNICName).
			WithNetwork(testNICName, "").Build()

		clientset := fake.NewSimpleClientset()
		err := clientset.Tracker().Add(givenVM)
		if err != nil {
			t.Fatal(err)
		}

		handler := Handler{
			vmnetcfgCache:  fakeclient.VirtualMachineNetworkConfigCache(clientset.NetworkV1alpha1().VirtualMachineNetworkConfigs),
			vmnetcfgClient: fakeclient.VirtualMachineNetworkConfigClient(clientset.NetworkV1alpha1().VirtualMachineNetworkConfigs),
		}

		_, err = handler.OnChange(testKey, givenVM)
		assert.Nil(t, err)

		_, err = handler.vmnetcfgClient.Get(testVmNetCfgNamespace, testVmNetCfgName, metav1.GetOptions{})
		assert.NotNil(t, err)
	})

	t.Run("vm and vmnetcfg network configs are in-sync", func(t *testing.T) {
		givenVM := newTestVMBuilder().
			WithInterfaceInAnnotation(testMACAddress1, testNICName).
			WithNetwork(testNICName, testNetworkName).Build()
		givenVmNetCfg := newTestVmNetCfgBuilder().
			Label(vmLabelKey, testVMName).
			WithVMName(testVMName).
			WithNetworkConfig("", testMACAddress1, testNetworkName).Build()

		clientset := fake.NewSimpleClientset()
		err := clientset.Tracker().Add(givenVM)
		if err != nil {
			t.Fatal(err)
		}
		err = clientset.Tracker().Add(givenVmNetCfg)
		if err != nil {
			t.Fatal(err)
		}

		handler := Handler{
			vmnetcfgCache:  fakeclient.VirtualMachineNetworkConfigCache(clientset.NetworkV1alpha1().VirtualMachineNetworkConfigs),
			vmnetcfgClient: fakeclient.VirtualMachineNetworkConfigClient(clientset.NetworkV1alpha1().VirtualMachineNetworkConfigs),
		}

		_, err = handler.OnChange(testKey, givenVM)
		assert.Nil(t, err)

		vmNetCfg, err := handler.vmnetcfgClient.Get(testVmNetCfgNamespace, testVmNetCfgName, metav1.GetOptions{})
		assert.Nil(t, err)
		assert.Equal(t, givenVmNetCfg, vmNetCfg)
	})

	t.Run("vm and vmnetcfg found inconsistent in network configs should be flagged (first iteration)", func(t *testing.T) {
		givenVM := newTestVMBuilder().
			WithInterfaceInAnnotation(testMACAddress2, testNICName).
			WithNetwork(testNICName, testNetworkName).Build()
		givenVmNetCfg := newTestVmNetCfgBuilder().
			Label(vmLabelKey, testVMName).
			WithVMName(testVMName).
			WithNetworkConfig("", testMACAddress1, testNetworkName).
			WithNetworkConfigStatus(testIPAddress, testMACAddress1, testNetworkName, networkv1.AllocatedState).
			InSyncedCondition(corev1.ConditionTrue, "", "").Build()

		expectedVmNetCfg := newTestVmNetCfgBuilder().
			Label(vmLabelKey, testVMName).
			WithVMName(testVMName).
			WithNetworkConfig("", testMACAddress1, testNetworkName).
			WithNetworkConfigStatus(testIPAddress, testMACAddress1, testNetworkName, networkv1.AllocatedState).
			InSyncedCondition(corev1.ConditionFalse, "NetworkConfigChanged", "Network configuration of the upstrem virtual machine has been changed").Build()

		clientset := fake.NewSimpleClientset()
		err := clientset.Tracker().Add(givenVM)
		if err != nil {
			t.Fatal(err)
		}
		err = clientset.Tracker().Add(givenVmNetCfg)
		if err != nil {
			t.Fatal(err)
		}

		handler := Handler{
			vmController:   fakecontroller.VirtualMachineController(clientset.KubevirtV1().VirtualMachines),
			vmnetcfgCache:  fakeclient.VirtualMachineNetworkConfigCache(clientset.NetworkV1alpha1().VirtualMachineNetworkConfigs),
			vmnetcfgClient: fakeclient.VirtualMachineNetworkConfigClient(clientset.NetworkV1alpha1().VirtualMachineNetworkConfigs),
		}

		_, err = handler.OnChange(testKey, givenVM)
		assert.Nil(t, err)

		vmNetCfg, err := handler.vmnetcfgClient.Get(testVmNetCfgNamespace, testVmNetCfgName, metav1.GetOptions{})
		assert.Nil(t, err)
		// The InSynced condition is the only thing we care about in this test
		assert.Equal(t, expectedVmNetCfg.Status, vmNetCfg.Status)
	})

	t.Run("flagged vm and vmnetcfg network configs inconsistency should be synced (second iteration)", func(t *testing.T) {
		givenVM := newTestVMBuilder().
			WithInterfaceInAnnotation(testMACAddress2, testNICName).
			WithNetwork(testNICName, testNetworkName).Build()
		givenVmNetCfg := newTestVmNetCfgBuilder().
			Label(vmLabelKey, testVMName).
			WithVMName(testVMName).
			WithNetworkConfig("", testMACAddress1, testNetworkName).
			InSyncedCondition(corev1.ConditionFalse, "NetworkConfigChanged", "Network configuration of the upstrem virtual machine has been changed").Build()

		expectedVmNetCfg := newTestVmNetCfgBuilder().
			Label(vmLabelKey, testVMName).
			WithVMName(testVMName).
			WithNetworkConfig("", testMACAddress2, testNetworkName).
			InSyncedCondition(corev1.ConditionFalse, "NetworkConfigChanged", "Network configuration of the upstrem virtual machine has been changed").Build()

		clientset := fake.NewSimpleClientset()
		err := clientset.Tracker().Add(givenVM)
		if err != nil {
			t.Fatal(err)
		}
		err = clientset.Tracker().Add(givenVmNetCfg)
		if err != nil {
			t.Fatal(err)
		}

		handler := Handler{
			vmController:   fakecontroller.VirtualMachineController(clientset.KubevirtV1().VirtualMachines),
			vmnetcfgCache:  fakeclient.VirtualMachineNetworkConfigCache(clientset.NetworkV1alpha1().VirtualMachineNetworkConfigs),
			vmnetcfgClient: fakeclient.VirtualMachineNetworkConfigClient(clientset.NetworkV1alpha1().VirtualMachineNetworkConfigs),
		}

		_, err = handler.OnChange(testKey, givenVM)
		assert.Nil(t, err)

		vmNetCfg, err := handler.vmnetcfgClient.Get(testVmNetCfgNamespace, testVmNetCfgName, metav1.GetOptions{})
		assert.Nil(t, err)
		// Spec should be synced with the VM
		assert.Equal(t, expectedVmNetCfg, vmNetCfg)
	})
}
