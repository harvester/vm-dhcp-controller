package ippool

import (
	"fmt"
	"net/netip"
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

func (v *Validator) Create(_ *admission.Request, newObj runtime.Object) error {
	ipPool := newObj.(*networkv1.IPPool)
	logrus.Infof("create ippool %s/%s", ipPool.Namespace, ipPool.Name)

	if err := v.checkNAD(ipPool); err != nil {
		return fmt.Errorf(webhook.CreateErr, ipPool.Kind, ipPool.Namespace, ipPool.Name, err)
	}

	if err := v.checkServerIP(ipPool); err != nil {
		return fmt.Errorf(webhook.CreateErr, ipPool.Kind, ipPool.Namespace, ipPool.Name, err)
	}

	return nil
}

func (v *Validator) Update(_ *admission.Request, _, newObj runtime.Object) error {
	ipPool := newObj.(*networkv1.IPPool)

	if ipPool.DeletionTimestamp != nil {
		return nil
	}

	logrus.Infof("update ippool %s/%s", ipPool.Namespace, ipPool.Name)

	if err := v.checkNAD(ipPool); err != nil {
		return fmt.Errorf(webhook.CreateErr, ipPool.Kind, ipPool.Namespace, ipPool.Name, err)
	}

	if err := v.checkServerIP(ipPool); err != nil {
		return fmt.Errorf(webhook.CreateErr, ipPool.Kind, ipPool.Namespace, ipPool.Name, err)
	}

	return nil
}

func (v *Validator) Delete(_ *admission.Request, oldObj runtime.Object) error {
	ipPool := oldObj.(*networkv1.IPPool)
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
			admissionregv1.Update,
			admissionregv1.Delete,
		},
	}
}

func (v *Validator) checkNAD(ipPool *networkv1.IPPool) error {
	nadNamespace, nadName := kv.RSplit(ipPool.Spec.NetworkName, "/")
	if nadNamespace == "" {
		nadNamespace = "default"
	}

	_, err := v.nadCache.Get(nadNamespace, nadName)
	return err
}

func (v *Validator) checkServerIP(ipPool *networkv1.IPPool) error {
	ipNet, networkIPAddr, broadcastIPAddr, err := util.LoadCIDR(ipPool.Spec.IPv4Config.CIDR)
	if err != nil {
		return err
	}

	routerIPAddr, err := netip.ParseAddr(ipPool.Spec.IPv4Config.Router)
	if err != nil {
		routerIPAddr = netip.Addr{}
	}

	serverIPAddr, err := netip.ParseAddr(ipPool.Spec.IPv4Config.ServerIP)
	if err != nil {
		return err
	}

	if !ipNet.Contains(serverIPAddr.AsSlice()) {
		return fmt.Errorf("server ip %s is not within subnet", serverIPAddr)
	}

	if serverIPAddr.As4() == networkIPAddr.As4() {
		return fmt.Errorf("server ip %s cannot be the same as network ip", serverIPAddr)
	}

	if serverIPAddr.As4() == broadcastIPAddr.As4() {
		return fmt.Errorf("server ip %s cannot be the same as broadcast ip", serverIPAddr)
	}

	if routerIPAddr.IsValid() && serverIPAddr.As4() == routerIPAddr.As4() {
		return fmt.Errorf("server ip %s cannot be the same as router ip", serverIPAddr)
	}

	if ipPool.Status.IPv4 != nil {
		for ip, mac := range ipPool.Status.IPv4.Allocated {
			if serverIPAddr.String() == ip {
				return fmt.Errorf("server ip %s is already allocated by mac %s", serverIPAddr, mac)
			}
		}
	}

	return nil
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
