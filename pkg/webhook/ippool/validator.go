package ippool

import (
	"fmt"
	"strings"

	"github.com/harvester/webhook/pkg/server/admission"
	"github.com/rancher/wrangler/pkg/kv"
	"github.com/sirupsen/logrus"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"

	networkv1 "github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	ctlcniv1 "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/k8s.cni.cncf.io/v1"
	ctlnetworkv1 "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/network.harvesterhci.io/v1alpha1"
	"github.com/harvester/vm-dhcp-controller/pkg/util"
	"github.com/harvester/vm-dhcp-controller/pkg/webhook"
)

type Validator struct {
	admission.DefaultValidator

	nadCache      ctlcniv1.NetworkAttachmentDefinitionCache
	vmnetcfgCache ctlnetworkv1.VirtualMachineNetworkConfigCache
}

func NewValidator(nadCache ctlcniv1.NetworkAttachmentDefinitionCache, vmnetcfgCache ctlnetworkv1.VirtualMachineNetworkConfigCache) *Validator {
	return &Validator{
		nadCache:      nadCache,
		vmnetcfgCache: vmnetcfgCache,
	}
}

func (v *Validator) Create(request *admission.Request, newObj runtime.Object) error {
	ipPool := newObj.(*networkv1.IPPool)
	logrus.Infof("create ippool %s/%s", ipPool.Namespace, ipPool.Name)

	nadNamespace, nadName := kv.RSplit(ipPool.Spec.NetworkName, "/")
	if nadNamespace == "" {
		nadNamespace = "default"
	}

	if _, err := v.nadCache.Get(nadNamespace, nadName); err != nil {
		return fmt.Errorf(webhook.CreateErr, ipPool.Kind, ipPool.Namespace, ipPool.Name, err)
	}

	return nil
}

func (v *Validator) Delete(request *admission.Request, newObj runtime.Object) error {
	ipPool := newObj.(*networkv1.IPPool)
	logrus.Infof("delete ippool %s/%s", ipPool.Namespace, ipPool.Name)

	if err := v.checkVmNetCfgs(ipPool); err != nil {
		return fmt.Errorf(webhook.DeleteErr, ipPool.Kind, ipPool.Namespace, ipPool.Name, err)
	}

	return nil
}

func (v *Validator) Resource() admission.Resource {
	return admission.Resource{
		Names:      []string{"ippools"},
		Scope:      admissionregv1.NamespacedScope,
		APIGroup:   networkv1.SchemeGroupVersion.Group,
		APIVersion: networkv1.SchemeGroupVersion.Version,
		ObjectType: &networkv1.IPPool{},
		OperationTypes: []admissionregv1.OperationType{
			admissionregv1.Create,
			admissionregv1.Delete,
		},
	}
}

func (v *Validator) checkVmNetCfgs(ipPool *networkv1.IPPool) error {
	vmnetcfgGetter := util.VmnetcfgGetter{
		VmnetcfgCache: v.vmnetcfgCache,
	}
	vmNetCfgs, err := vmnetcfgGetter.WhoUseIPPool(ipPool)
	if err != nil {
		return err
	}

	logrus.Infof("%d vmnetcfg(s) associated", len(vmNetCfgs))

	if len(vmNetCfgs) > 0 {
		vmNetCfgNames := make([]string, 0, len(vmNetCfgs))
		for _, vmNetCfg := range vmNetCfgs {
			vmNetCfgNames = append(vmNetCfgNames, vmNetCfg.Namespace+"/"+vmNetCfg.Name)
		}
		return fmt.Errorf("it's still used by VirtualMachineNetworkConfig(s) %s, which must be removed at first", strings.Join(vmNetCfgNames, ", "))
	}
	return nil
}
