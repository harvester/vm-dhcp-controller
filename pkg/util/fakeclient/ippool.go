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

type IPPoolClient func(string) typenetworkv1.IPPoolInterface

func (c IPPoolClient) Update(ipPool *networkv1.IPPool) (*networkv1.IPPool, error) {
	return c(ipPool.Namespace).Update(context.TODO(), ipPool, metav1.UpdateOptions{})
}
func (c IPPoolClient) Get(namespace, name string, options metav1.GetOptions) (*networkv1.IPPool, error) {
	return c(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}
func (c IPPoolClient) Create(*networkv1.IPPool) (*networkv1.IPPool, error) {
	panic("implement me")
}
func (c IPPoolClient) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	panic("implement me")
}
func (c IPPoolClient) List(namespace string, opts metav1.ListOptions) (*networkv1.IPPoolList, error) {
	panic("implement me")
}
func (c IPPoolClient) UpdateStatus(ipPool *networkv1.IPPool) (*networkv1.IPPool, error) {
	return c(ipPool.Namespace).UpdateStatus(context.TODO(), ipPool, metav1.UpdateOptions{})
}
func (c IPPoolClient) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}
func (c IPPoolClient) Patch(namespace, name string, pt types.PatchType, data []byte, subresources ...string) (result *networkv1.IPPool, err error) {
	panic("implement me")
}

type IPPoolCache func(string) typenetworkv1.IPPoolInterface

func (c IPPoolCache) Get(namespace, name string) (*networkv1.IPPool, error) {
	return c(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}
func (c IPPoolCache) List(namespace string, selector labels.Selector) ([]*networkv1.IPPool, error) {
	list, err := c(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	result := make([]*networkv1.IPPool, 0, len(list.Items))
	for _, ipPool := range list.Items {
		result = append(result, &ipPool)
	}
	return result, err
}
func (c IPPoolCache) AddIndexer(indexName string, indexer ctlnetworkv1.IPPoolIndexer) {
	panic("implement me")
}
func (c IPPoolCache) GetByIndex(indexName, key string) ([]*networkv1.IPPool, error) {
	panic("implement me")
}
