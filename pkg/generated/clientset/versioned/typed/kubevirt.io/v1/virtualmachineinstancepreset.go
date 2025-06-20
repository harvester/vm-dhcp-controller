/*
Copyright 2025 Rancher Labs, Inc.

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

package v1

import (
	"context"

	scheme "github.com/harvester/vm-dhcp-controller/pkg/generated/clientset/versioned/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	gentype "k8s.io/client-go/gentype"
	v1 "kubevirt.io/api/core/v1"
)

// VirtualMachineInstancePresetsGetter has a method to return a VirtualMachineInstancePresetInterface.
// A group's client should implement this interface.
type VirtualMachineInstancePresetsGetter interface {
	VirtualMachineInstancePresets(namespace string) VirtualMachineInstancePresetInterface
}

// VirtualMachineInstancePresetInterface has methods to work with VirtualMachineInstancePreset resources.
type VirtualMachineInstancePresetInterface interface {
	Create(ctx context.Context, virtualMachineInstancePreset *v1.VirtualMachineInstancePreset, opts metav1.CreateOptions) (*v1.VirtualMachineInstancePreset, error)
	Update(ctx context.Context, virtualMachineInstancePreset *v1.VirtualMachineInstancePreset, opts metav1.UpdateOptions) (*v1.VirtualMachineInstancePreset, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.VirtualMachineInstancePreset, error)
	List(ctx context.Context, opts metav1.ListOptions) (*v1.VirtualMachineInstancePresetList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.VirtualMachineInstancePreset, err error)
	VirtualMachineInstancePresetExpansion
}

// virtualMachineInstancePresets implements VirtualMachineInstancePresetInterface
type virtualMachineInstancePresets struct {
	*gentype.ClientWithList[*v1.VirtualMachineInstancePreset, *v1.VirtualMachineInstancePresetList]
}

// newVirtualMachineInstancePresets returns a VirtualMachineInstancePresets
func newVirtualMachineInstancePresets(c *KubevirtV1Client, namespace string) *virtualMachineInstancePresets {
	return &virtualMachineInstancePresets{
		gentype.NewClientWithList[*v1.VirtualMachineInstancePreset, *v1.VirtualMachineInstancePresetList](
			"virtualmachineinstancepresets",
			c.RESTClient(),
			scheme.ParameterCodec,
			namespace,
			func() *v1.VirtualMachineInstancePreset { return &v1.VirtualMachineInstancePreset{} },
			func() *v1.VirtualMachineInstancePresetList { return &v1.VirtualMachineInstancePresetList{} }),
	}
}
