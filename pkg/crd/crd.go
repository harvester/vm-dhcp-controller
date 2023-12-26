package crd

import (
	"context"

	"github.com/rancher/wrangler/pkg/crd"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"

	network "github.com/starbops/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
)

var ageColumn = apiextv1.CustomResourceColumnDefinition{
	Name:     "AGE",
	Type:     "date",
	Priority: 10,
	JSONPath: ".metadata.creationTimestamp",
}

func List() []crd.CRD {
	return []crd.CRD{
		newCRD("network.harvesterhci.io", &network.IPPool{}, func(c crd.CRD) crd.CRD {
			return c.
				WithShortNames("ipl", "ipls").
				WithColumn("NETWORK", ".spec.networkName").
				WithColumn("AVAILABLE", ".status.ipv4.available").
				WithColumn("USED", ".status.ipv4.used").
				WithCustomColumn(ageColumn)
		}),
		newCRD("network.harvesterhci.io", &network.VirtualMachineNetworkConfig{}, func(c crd.CRD) crd.CRD {
			return c.
				WithShortNames("vmnetcfg", "vmnetcfgs").
				WithColumn("VMNAME", ".spec.vmName").
				WithColumn("NETWORK", ".spec.networkConfig.networkName").
				WithCustomColumn(ageColumn)
		}),
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
			Version: "v1alpha1",
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
