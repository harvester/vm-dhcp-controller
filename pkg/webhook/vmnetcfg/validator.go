package vmnetcfg

import (
	"fmt"

	admissionregv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"

	networkv1 "github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	ctlcniv1 "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/k8s.cni.cncf.io/v1"
	ctlnetworkv1 "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/network.harvesterhci.io/v1alpha1"
	"github.com/harvester/vm-dhcp-controller/pkg/util"
	"github.com/harvester/vm-dhcp-controller/pkg/webhook"
	"github.com/harvester/webhook/pkg/server/admission"
	"github.com/rancher/wrangler/v3/pkg/kv"
	"github.com/sirupsen/logrus"
)

type Validator struct {
	admission.DefaultValidator

	ippoolCache ctlnetworkv1.IPPoolCache
	nadCache    ctlcniv1.NetworkAttachmentDefinitionCache
}

func NewValidator(ippoolCache ctlnetworkv1.IPPoolCache, nadCache ctlcniv1.NetworkAttachmentDefinitionCache) *Validator {
	return &Validator{
		ippoolCache: ippoolCache,
		nadCache:    nadCache,
	}
}

func (v *Validator) Create(request *admission.Request, newObj runtime.Object) error {
	vmNetCfg := newObj.(*networkv1.VirtualMachineNetworkConfig)
	logrus.Infof("create vmnetcfg %s/%s", vmNetCfg.Namespace, vmNetCfg.Name)

	for _, nc := range vmNetCfg.Spec.NetworkConfigs {
		nadNamespace, nadName := kv.RSplit(nc.NetworkName, "/")
		if nadNamespace == "" {
			nadNamespace = "default"
		}
		nad, err := v.nadCache.Get(nadNamespace, nadName)
		if err != nil {
			return fmt.Errorf(webhook.CreateErr, vmNetCfg.Kind, vmNetCfg.Namespace, vmNetCfg.Name, err)
		}
		ipPoolNamespace, ok := nad.Labels[util.IPPoolNamespaceLabelKey]
		if !ok {
			return fmt.Errorf(
				webhook.CreateErr,
				vmNetCfg.Kind,
				vmNetCfg.Namespace,
				vmNetCfg.Name,
				fmt.Errorf("%s label not found", util.IPPoolNamespaceLabelKey),
			)
		}
		ipPoolName, ok := nad.Labels[util.IPPoolNameLabelKey]
		if !ok {
			return fmt.Errorf(
				webhook.CreateErr,
				vmNetCfg.Kind,
				vmNetCfg.Namespace,
				vmNetCfg.Name, fmt.Errorf("%s label not found", util.IPPoolNameLabelKey),
			)
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
