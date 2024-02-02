package vmnetcfg

import (
	"fmt"

	admissionregv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"

	networkv1 "github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	ctlnetworkv1 "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/network.harvesterhci.io/v1alpha1"
	"github.com/harvester/vm-dhcp-controller/pkg/webhook"
	"github.com/harvester/webhook/pkg/server/admission"
	"github.com/rancher/wrangler/pkg/kv"
	"github.com/sirupsen/logrus"
)

type Validator struct {
	admission.DefaultValidator

	ippoolCache ctlnetworkv1.IPPoolCache
}

func NewValidator(ippoolCache ctlnetworkv1.IPPoolCache) *Validator {
	return &Validator{
		ippoolCache: ippoolCache,
	}
}

func (v *Validator) Create(request *admission.Request, newObj runtime.Object) error {
	vmNetCfg := newObj.(*networkv1.VirtualMachineNetworkConfig)
	logrus.Infof("create vmnetcfg %s/%s", vmNetCfg.Namespace, vmNetCfg.Name)

	for _, nc := range vmNetCfg.Spec.NetworkConfigs {
		ipPoolNamespace, ipPoolName := kv.RSplit(nc.NetworkName, "/")
		if ipPoolNamespace == "" {
			ipPoolNamespace = "default"
		}
		if _, err := v.ippoolCache.Get(ipPoolNamespace, ipPoolName); err != nil {
			return fmt.Errorf(webhook.CreateErr, vmNetCfg.Kind, vmNetCfg.Namespace, vmNetCfg.Name, err)
		}
	}

	return nil
}

func (v *Validator) Resource() admission.Resource {
	return admission.Resource{
		Names:      []string{"virtualmachinenetworkconfigs"},
		Scope:      admissionregv1.NamespacedScope,
		APIGroup:   networkv1.SchemeGroupVersion.Group,
		APIVersion: networkv1.SchemeGroupVersion.Version,
		ObjectType: &networkv1.VirtualMachineNetworkConfig{},
		OperationTypes: []admissionregv1.OperationType{
			admissionregv1.Create,
		},
	}
}
