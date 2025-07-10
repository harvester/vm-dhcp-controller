package fakecontroller

import (
	"context"
	"time"

	"github.com/rancher/wrangler/v3/pkg/generic"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	kubevirtv1 "kubevirt.io/api/core/v1"

	typekubevirtv1 "github.com/harvester/vm-dhcp-controller/pkg/generated/clientset/versioned/typed/kubevirt.io/v1"
)

type VirtualMachineController func(string) typekubevirtv1.VirtualMachineInterface

func (c VirtualMachineController) Informer() cache.SharedIndexInformer {
	panic("implement me")
}
func (c VirtualMachineController) GroupVersionKind() schema.GroupVersionKind {
	panic("implement me")
}
func (c VirtualMachineController) AddGenericHandler(ctx context.Context, name string, handler generic.Handler) {
	panic("implement me")
}
func (c VirtualMachineController) AddGenericRemoveHandler(ctx context.Context, name string, handler generic.Handler) {
	panic("implement me")
}
func (c VirtualMachineController) Updater() generic.Updater {
	panic("implement me")
}

func (c VirtualMachineController) Create(*kubevirtv1.VirtualMachine) (*kubevirtv1.VirtualMachine, error) {
	panic("implement me")
}
func (c VirtualMachineController) Update(*kubevirtv1.VirtualMachine) (*kubevirtv1.VirtualMachine, error) {
	panic("implement me")
}
func (c VirtualMachineController) UpdateStatus(*kubevirtv1.VirtualMachine) (*kubevirtv1.VirtualMachine, error) {
	panic("implement me")
}
func (c VirtualMachineController) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	panic("implement me")
}
func (c VirtualMachineController) Get(namespace, name string, options metav1.GetOptions) (*kubevirtv1.VirtualMachine, error) {
	panic("implement me")
}
func (c VirtualMachineController) List(namespace string, opts metav1.ListOptions) (*kubevirtv1.VirtualMachineList, error) {
	panic("implement me")
}
func (c VirtualMachineController) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}
func (c VirtualMachineController) Patch(namespace, name string, pt types.PatchType, data []byte, subresources ...string) (result *kubevirtv1.VirtualMachine, err error) {
	panic("implement me")
}
func (c VirtualMachineController) WithImpersonation(impersonate rest.ImpersonationConfig) (generic.ClientInterface[*kubevirtv1.VirtualMachine, *kubevirtv1.VirtualMachineList], error) {
	panic("implement me")
}

func (c VirtualMachineController) OnChange(ctx context.Context, name string, _ generic.ObjectHandler[*kubevirtv1.VirtualMachine]) {
	panic("implement me")
}
func (c VirtualMachineController) OnRemove(ctx context.Context, name string, _ generic.ObjectHandler[*kubevirtv1.VirtualMachine]) {
	panic("implement me")
}
func (c VirtualMachineController) Enqueue(namespace, name string) {}
func (c VirtualMachineController) EnqueueAfter(namespace, name string, duration time.Duration) {
	panic("implement me")
}
func (c VirtualMachineController) Cache() generic.CacheInterface[*kubevirtv1.VirtualMachine] {
	panic("implement me")
}
