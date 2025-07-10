package fakeclient

import (
	"context"

	"github.com/rancher/wrangler/v3/pkg/generic"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	kubevirtv1 "kubevirt.io/api/core/v1"

	typekubevirtv1 "github.com/harvester/vm-dhcp-controller/pkg/generated/clientset/versioned/typed/kubevirt.io/v1"
)

type VirtualMachineClient func(string) typekubevirtv1.VirtualMachineInterface

func (c VirtualMachineClient) Update(nad *kubevirtv1.VirtualMachine) (*kubevirtv1.VirtualMachine, error) {
	return c(nad.Namespace).Update(context.TODO(), nad, metav1.UpdateOptions{})
}
func (c VirtualMachineClient) Get(namespace, name string, options metav1.GetOptions) (*kubevirtv1.VirtualMachine, error) {
	return c(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}
func (c VirtualMachineClient) Create(nad *kubevirtv1.VirtualMachine) (*kubevirtv1.VirtualMachine, error) {
	return c(nad.Namespace).Create(context.TODO(), nad, metav1.CreateOptions{})
}
func (c VirtualMachineClient) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return c(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}
func (c VirtualMachineClient) List(namespace string, opts metav1.ListOptions) (*kubevirtv1.VirtualMachineList, error) {
	panic("implement me")
}
func (c VirtualMachineClient) UpdateStatus(nad *kubevirtv1.VirtualMachine) (*kubevirtv1.VirtualMachine, error) {
	panic("implement me")
}
func (c VirtualMachineClient) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}
func (c VirtualMachineClient) Patch(namespace, name string, pt types.PatchType, data []byte, subresources ...string) (result *kubevirtv1.VirtualMachine, err error) {
	panic("implement me")
}

func (c VirtualMachineClient) WithImpersonation(config rest.ImpersonationConfig) (generic.ClientInterface[*kubevirtv1.VirtualMachine, *kubevirtv1.VirtualMachineList], error) {
	panic("implement me")
}

type VirtualMachineCache func(string) typekubevirtv1.VirtualMachineInterface

func (c VirtualMachineCache) Get(namespace, name string) (*kubevirtv1.VirtualMachine, error) {
	return c(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}
func (c VirtualMachineCache) List(namespace string, selector labels.Selector) ([]*kubevirtv1.VirtualMachine, error) {
	list, err := c(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	result := make([]*kubevirtv1.VirtualMachine, 0, len(list.Items))
	for _, nad := range list.Items {
		n := nad
		result = append(result, &n)
	}
	return result, err
}
func (c VirtualMachineCache) AddIndexer(indexName string, indexer generic.Indexer[*kubevirtv1.VirtualMachine]) {
	panic("implement me")
}
func (c VirtualMachineCache) GetByIndex(indexName, key string) ([]*kubevirtv1.VirtualMachine, error) {
	panic("implement me")
}
