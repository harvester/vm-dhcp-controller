/*
Copyright 2023 Rancher Labs, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by main. DO NOT EDIT.

package fake

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
	v1 "kubevirt.io/api/core/v1"
)

// FakeVirtualMachineInstances implements VirtualMachineInstanceInterface
type FakeVirtualMachineInstances struct {
	Fake *FakeKubevirtV1
	ns   string
}

var virtualmachineinstancesResource = v1.SchemeGroupVersion.WithResource("virtualmachineinstances")

var virtualmachineinstancesKind = v1.SchemeGroupVersion.WithKind("VirtualMachineInstance")

// Get takes name of the virtualMachineInstance, and returns the corresponding virtualMachineInstance object, and an error if there is any.
func (c *FakeVirtualMachineInstances) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.VirtualMachineInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(virtualmachineinstancesResource, c.ns, name), &v1.VirtualMachineInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.VirtualMachineInstance), err
}

// List takes label and field selectors, and returns the list of VirtualMachineInstances that match those selectors.
func (c *FakeVirtualMachineInstances) List(ctx context.Context, opts metav1.ListOptions) (result *v1.VirtualMachineInstanceList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(virtualmachineinstancesResource, virtualmachineinstancesKind, c.ns, opts), &v1.VirtualMachineInstanceList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.VirtualMachineInstanceList{ListMeta: obj.(*v1.VirtualMachineInstanceList).ListMeta}
	for _, item := range obj.(*v1.VirtualMachineInstanceList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested virtualMachineInstances.
func (c *FakeVirtualMachineInstances) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(virtualmachineinstancesResource, c.ns, opts))

}

// Create takes the representation of a virtualMachineInstance and creates it.  Returns the server's representation of the virtualMachineInstance, and an error, if there is any.
func (c *FakeVirtualMachineInstances) Create(ctx context.Context, virtualMachineInstance *v1.VirtualMachineInstance, opts metav1.CreateOptions) (result *v1.VirtualMachineInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(virtualmachineinstancesResource, c.ns, virtualMachineInstance), &v1.VirtualMachineInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.VirtualMachineInstance), err
}

// Update takes the representation of a virtualMachineInstance and updates it. Returns the server's representation of the virtualMachineInstance, and an error, if there is any.
func (c *FakeVirtualMachineInstances) Update(ctx context.Context, virtualMachineInstance *v1.VirtualMachineInstance, opts metav1.UpdateOptions) (result *v1.VirtualMachineInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(virtualmachineinstancesResource, c.ns, virtualMachineInstance), &v1.VirtualMachineInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.VirtualMachineInstance), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeVirtualMachineInstances) UpdateStatus(ctx context.Context, virtualMachineInstance *v1.VirtualMachineInstance, opts metav1.UpdateOptions) (*v1.VirtualMachineInstance, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(virtualmachineinstancesResource, "status", c.ns, virtualMachineInstance), &v1.VirtualMachineInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.VirtualMachineInstance), err
}

// Delete takes name of the virtualMachineInstance and deletes it. Returns an error if one occurs.
func (c *FakeVirtualMachineInstances) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(virtualmachineinstancesResource, c.ns, name, opts), &v1.VirtualMachineInstance{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeVirtualMachineInstances) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(virtualmachineinstancesResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1.VirtualMachineInstanceList{})
	return err
}

// Patch applies the patch and returns the patched virtualMachineInstance.
func (c *FakeVirtualMachineInstances) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.VirtualMachineInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(virtualmachineinstancesResource, c.ns, name, pt, data, subresources...), &v1.VirtualMachineInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.VirtualMachineInstance), err
}
