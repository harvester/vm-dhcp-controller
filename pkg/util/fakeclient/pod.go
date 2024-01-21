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

type PodClient func(string) typecorev1.PodInterface

func (c PodClient) Update(pod *corev1.Pod) (*corev1.Pod, error) {
	return c(pod.Namespace).Update(context.TODO(), pod, metav1.UpdateOptions{})
}
func (c PodClient) Get(namespace, name string, options metav1.GetOptions) (*corev1.Pod, error) {
	return c(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}
func (c PodClient) Create(pod *corev1.Pod) (*corev1.Pod, error) {
	return c(pod.Namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
}
func (c PodClient) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return c(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}
func (c PodClient) List(namespace string, opts metav1.ListOptions) (*corev1.PodList, error) {
	panic("implement me")
}
func (c PodClient) UpdateStatus(pod *corev1.Pod) (*corev1.Pod, error) {
	return c(pod.Namespace).UpdateStatus(context.TODO(), pod, metav1.UpdateOptions{})
}
func (c PodClient) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}
func (c PodClient) Patch(namespace, name string, pt types.PatchType, data []byte, subresources ...string) (result *corev1.Pod, err error) {
	panic("implement me")
}

type PodCache func(string) typecorev1.PodInterface

func (c PodCache) Get(namespace, name string) (*corev1.Pod, error) {
	return c(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}
func (c PodCache) List(namespace string, selector labels.Selector) ([]*corev1.Pod, error) {
	list, err := c(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	result := make([]*corev1.Pod, 0, len(list.Items))
	for _, pod := range list.Items {
		result = append(result, &pod)
	}
	return result, err
}
func (c PodCache) AddIndexer(indexName string, indexer ctlcorev1.PodIndexer) {
	panic("implement me")
}
func (c PodCache) GetByIndex(indexName, key string) ([]*corev1.Pod, error) {
	panic("implement me")
}
