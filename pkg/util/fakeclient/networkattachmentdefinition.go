package fakeclient

import (
	"context"

	cniv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"

	typecniv1 "github.com/harvester/vm-dhcp-controller/pkg/generated/clientset/versioned/typed/k8s.cni.cncf.io/v1"
	ctlcniv1 "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/k8s.cni.cncf.io/v1"
)

type NetworkAttachmentDefinitionClient func(string) typecniv1.NetworkAttachmentDefinitionInterface

func (c NetworkAttachmentDefinitionClient) Update(nad *cniv1.NetworkAttachmentDefinition) (*cniv1.NetworkAttachmentDefinition, error) {
	return c(nad.Namespace).Update(context.TODO(), nad, metav1.UpdateOptions{})
}
func (c NetworkAttachmentDefinitionClient) Get(namespace, name string, options metav1.GetOptions) (*cniv1.NetworkAttachmentDefinition, error) {
	panic("implement me")
}
func (c NetworkAttachmentDefinitionClient) Create(nad *cniv1.NetworkAttachmentDefinition) (*cniv1.NetworkAttachmentDefinition, error) {
	return c(nad.Namespace).Create(context.TODO(), nad, metav1.CreateOptions{})
}
func (c NetworkAttachmentDefinitionClient) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return c(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}
func (c NetworkAttachmentDefinitionClient) List(namespace string, opts metav1.ListOptions) (*cniv1.NetworkAttachmentDefinitionList, error) {
	panic("implement me")
}
func (c NetworkAttachmentDefinitionClient) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}
func (c NetworkAttachmentDefinitionClient) Patch(namespace, name string, pt types.PatchType, data []byte, subresources ...string) (result *cniv1.NetworkAttachmentDefinition, err error) {
	panic("implement me")
}

type NetworkAttachmentDefinitionCache func(string) typecniv1.NetworkAttachmentDefinitionInterface

func (c NetworkAttachmentDefinitionCache) Get(namespace, name string) (*cniv1.NetworkAttachmentDefinition, error) {
	return c(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}
func (c NetworkAttachmentDefinitionCache) List(namespace string, selector labels.Selector) ([]*cniv1.NetworkAttachmentDefinition, error) {
	list, err := c(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	result := make([]*cniv1.NetworkAttachmentDefinition, 0, len(list.Items))
	for _, nad := range list.Items {
		n := nad
		result = append(result, &n)
	}
	return result, err
}
func (c NetworkAttachmentDefinitionCache) AddIndexer(indexName string, indexer ctlcniv1.NetworkAttachmentDefinitionIndexer) {
	panic("implement me")
}
func (c NetworkAttachmentDefinitionCache) GetByIndex(indexName, key string) ([]*cniv1.NetworkAttachmentDefinition, error) {
	panic("implement me")
}
