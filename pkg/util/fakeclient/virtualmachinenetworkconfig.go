package fakeclient

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"

	networkv1 "github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	typenetworkv1 "github.com/harvester/vm-dhcp-controller/pkg/generated/clientset/versioned/typed/network.harvesterhci.io/v1alpha1"
	ctlnetworkv1 "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/network.harvesterhci.io/v1alpha1"
)

type VirtualMachineNetworkConfigClient func(string) typenetworkv1.VirtualMachineNetworkConfigInterface

func (c VirtualMachineNetworkConfigClient) Update(vmNetCfg *networkv1.VirtualMachineNetworkConfig) (*networkv1.VirtualMachineNetworkConfig, error) {
	return c(vmNetCfg.Namespace).Update(context.TODO(), vmNetCfg, metav1.UpdateOptions{})
}
func (c VirtualMachineNetworkConfigClient) Get(namespace, name string, options metav1.GetOptions) (*networkv1.VirtualMachineNetworkConfig, error) {
	panic("implement me")
}
func (c VirtualMachineNetworkConfigClient) Create(*networkv1.VirtualMachineNetworkConfig) (*networkv1.VirtualMachineNetworkConfig, error) {
	panic("implement me")
}
func (c VirtualMachineNetworkConfigClient) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	panic("implement me")
}
func (c VirtualMachineNetworkConfigClient) List(namespace string, opts metav1.ListOptions) (*networkv1.VirtualMachineNetworkConfigList, error) {
	panic("implement me")
}
func (c VirtualMachineNetworkConfigClient) UpdateStatus(vmNetCfg *networkv1.VirtualMachineNetworkConfig) (*networkv1.VirtualMachineNetworkConfig, error) {
	return c(vmNetCfg.Namespace).UpdateStatus(context.TODO(), vmNetCfg, metav1.UpdateOptions{})
}
func (c VirtualMachineNetworkConfigClient) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}
func (c VirtualMachineNetworkConfigClient) Patch(namespace, name string, pt types.PatchType, data []byte, subresources ...string) (result *networkv1.VirtualMachineNetworkConfig, err error) {
	panic("implement me")
}

type VirtualMachineNetworkConfigCache func(string) typenetworkv1.VirtualMachineNetworkConfigInterface

func (c VirtualMachineNetworkConfigCache) Get(namespace, name string) (*networkv1.VirtualMachineNetworkConfig, error) {
	return c(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}
func (c VirtualMachineNetworkConfigCache) List(namespace string, selector labels.Selector) ([]*networkv1.VirtualMachineNetworkConfig, error) {
	list, err := c(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	result := make([]*networkv1.VirtualMachineNetworkConfig, 0, len(list.Items))
	for _, vmNetCfg := range list.Items {
		v := vmNetCfg
		result = append(result, &v)
	}
	return result, err
}
func (c VirtualMachineNetworkConfigCache) AddIndexer(indexName string, indexer ctlnetworkv1.VirtualMachineNetworkConfigIndexer) {
	panic("implement me")
}
func (c VirtualMachineNetworkConfigCache) GetByIndex(indexName, key string) ([]*networkv1.VirtualMachineNetworkConfig, error) {
	panic("implement me")
}
