package crd

import (
	"context"

	"github.com/rancher/wrangler/pkg/crd"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"

	network "github.com/starbops/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
)

func List() []crd.CRD {
	return []crd.CRD{
		newCRD("network.harvesterhci.io", &network.IPPool{}, func(c crd.CRD) crd.CRD {
			return c.
				WithColumn("Available", ".status.ipv4.available").
				WithColumn("Used", ".status.ipv4.used")
		}),
		newCRD("network.harvesterhci.io", &network.VirtualMachineNetworkConfig{}, nil),
	}
}

func Create(ctx context.Context, cfg *rest.Config) error {
	factory, err := crd.NewFactoryFromClient(cfg)
	if err != nil {
		return err
	}

	return factory.BatchCreateCRDs(ctx, List()...).BatchWait()
}

func newCRD(group string, obj interface{}, customize func(crd.CRD) crd.CRD) crd.CRD {
	crd := crd.CRD{
		GVK: schema.GroupVersionKind{
			Group:   group,
			Version: "v1beta1",
		},
		Status:       true,
		NonNamespace: false,
		SchemaObject: obj,
	}
	if customize != nil {
		crd = customize(crd)
	}
	return crd
}
