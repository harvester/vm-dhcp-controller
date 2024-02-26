package fakeclient

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	typecorev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	ctlcorev1 "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/core/v1"
)

type NodeClient func() typecorev1.NodeInterface

func (c NodeClient) Update(node *corev1.Node) (*corev1.Node, error) {
	panic("implement me")
}
func (c NodeClient) Get(name string, options metav1.GetOptions) (*corev1.Node, error) {
	panic("implement me")
}
func (c NodeClient) Create(node *corev1.Node) (*corev1.Node, error) {
	panic("implement me")
}
func (c NodeClient) Delete(name string, options *metav1.DeleteOptions) error {
	panic("implement me")
}
func (c NodeClient) List(opts metav1.ListOptions) (*corev1.NodeList, error) {
	panic("implement me")
}
func (c NodeClient) UpdateStatus(node *corev1.Node) (*corev1.Node, error) {
	panic("implement me")
}
func (c NodeClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}
func (c NodeClient) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *corev1.Node, err error) {
	panic("implement me")
}

type NodeCache func() typecorev1.NodeInterface

func (c NodeCache) Get(name string) (*corev1.Node, error) {
	panic("implement me")
}
func (c NodeCache) List(selector labels.Selector) ([]*corev1.Node, error) {
	list, err := c().List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	result := make([]*corev1.Node, 0, len(list.Items))
	for _, node := range list.Items {
		n := node
		result = append(result, &n)
	}
	return result, err
}
func (c NodeCache) AddIndexer(indexName string, indexer ctlcorev1.NodeIndexer) {
	panic("implement me")
}
func (c NodeCache) GetByIndex(indexName, key string) ([]*corev1.Node, error) {
	panic("implement me")
}
