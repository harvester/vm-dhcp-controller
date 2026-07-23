package vmnetcfg

import (
	"fmt"
	"net/netip"

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

	return v.validate(vmNetCfg, webhook.CreateErr)
}

func (v *Validator) Update(_ *admission.Request, oldObj, newObj runtime.Object) error {
	vmNetCfg := newObj.(*networkv1.VirtualMachineNetworkConfig)
	if vmNetCfg.DeletionTimestamp != nil {
		return nil
	}

	logrus.Infof("update vmnetcfg %s/%s", vmNetCfg.Namespace, vmNetCfg.Name)

	oldVmNetCfg, ok := oldObj.(*networkv1.VirtualMachineNetworkConfig)
	if !ok || oldVmNetCfg == nil {
		return v.validate(vmNetCfg, webhook.UpdateErr)
	}

	return v.validateUpdate(oldVmNetCfg, vmNetCfg, webhook.UpdateErr)
}

func (v *Validator) validate(vmNetCfg *networkv1.VirtualMachineNetworkConfig, errFormat string) error {
	seenDesiredIPs := map[string]struct{}{}

	for _, nc := range vmNetCfg.Spec.NetworkConfigs {
		ipPool, networkName, err := v.getIPPoolFromNetworkConfig(nc)
		if err != nil {
			return fmt.Errorf(errFormat, vmNetCfg.Kind, vmNetCfg.Namespace, vmNetCfg.Name, err)
		}

		if nc.IPAddress == nil {
			continue
		}

		if err := validateDesiredIPAddress(nc, ipPool, networkName, seenDesiredIPs); err != nil {
			return fmt.Errorf(errFormat, vmNetCfg.Kind, vmNetCfg.Namespace, vmNetCfg.Name, err)
		}
	}

	return nil
}

func (v *Validator) validateUpdate(
	oldVmNetCfg, vmNetCfg *networkv1.VirtualMachineNetworkConfig,
	errFormat string,
) error {
	oldNetworkConfigs := map[util.NetworkConfigKey]networkv1.NetworkConfig{}
	for _, nc := range oldVmNetCfg.Spec.NetworkConfigs {
		oldNetworkConfigs[networkConfigKeyFromNetworkConfig(nc)] = nc
	}

	seenDesiredIPs := map[string]struct{}{}
	var networkConfigsToValidate []networkv1.NetworkConfig
	for _, nc := range vmNetCfg.Spec.NetworkConfigs {
		if nc.IPAddress == nil {
			continue
		}

		if desiredIPAddressChanged(oldNetworkConfigs, nc) {
			networkConfigsToValidate = append(networkConfigsToValidate, nc)
			continue
		}

		ipAddr, err := netip.ParseAddr(*nc.IPAddress)
		if err != nil {
			continue
		}
		seenDesiredIPs[desiredIPSeenKey(normalizeNetworkName(nc.NetworkName), ipAddr.String())] = struct{}{}
	}

	for _, nc := range networkConfigsToValidate {
		ipPool, networkName, err := v.getIPPoolFromNetworkConfig(nc)
		if err != nil {
			return fmt.Errorf(errFormat, vmNetCfg.Kind, vmNetCfg.Namespace, vmNetCfg.Name, err)
		}

		if err := validateDesiredIPAddress(nc, ipPool, networkName, seenDesiredIPs); err != nil {
			return fmt.Errorf(errFormat, vmNetCfg.Kind, vmNetCfg.Namespace, vmNetCfg.Name, err)
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
			admissionregv1.Update,
		},
	}
}

func (v *Validator) getIPPoolFromNetworkConfig(nc networkv1.NetworkConfig) (*networkv1.IPPool, string, error) {
	nadNamespace, nadName := kv.RSplit(nc.NetworkName, "/")
	if nadNamespace == "" {
		nadNamespace = "default"
	}
	networkName := nadNamespace + "/" + nadName

	nad, err := v.nadCache.Get(nadNamespace, nadName)
	if err != nil {
		return nil, networkName, err
	}

	ipPoolNamespace, ok := nad.Labels[util.IPPoolNamespaceLabelKey]
	if !ok {
		return nil, networkName, fmt.Errorf("%s label not found", util.IPPoolNamespaceLabelKey)
	}
	ipPoolName, ok := nad.Labels[util.IPPoolNameLabelKey]
	if !ok {
		return nil, networkName, fmt.Errorf("%s label not found", util.IPPoolNameLabelKey)
	}

	ipPool, err := v.ippoolCache.Get(ipPoolNamespace, ipPoolName)
	return ipPool, networkName, err
}

func networkConfigKeyFromNetworkConfig(nc networkv1.NetworkConfig) util.NetworkConfigKey {
	return util.NetworkConfigKey{
		NetworkName: normalizeNetworkName(nc.NetworkName),
		MACAddress:  nc.MACAddress,
	}
}

func normalizeNetworkName(networkName string) string {
	nadNamespace, nadName := kv.RSplit(networkName, "/")
	if nadNamespace == "" {
		nadNamespace = "default"
	}
	return nadNamespace + "/" + nadName
}

func desiredIPAddressChanged(oldNetworkConfigs map[util.NetworkConfigKey]networkv1.NetworkConfig, nc networkv1.NetworkConfig) bool {
	oldNC, ok := oldNetworkConfigs[networkConfigKeyFromNetworkConfig(nc)]
	if !ok {
		return true
	}
	if oldNC.IPAddress == nil || nc.IPAddress == nil {
		return oldNC.IPAddress != nc.IPAddress
	}
	return *oldNC.IPAddress != *nc.IPAddress
}

func desiredIPSeenKey(networkName, ipAddress string) string {
	return networkName + "/" + ipAddress
}

func validateDesiredIPAddress(
	nc networkv1.NetworkConfig,
	ipPool *networkv1.IPPool,
	networkName string,
	seenDesiredIPs map[string]struct{},
) error {
	ipAddr, err := netip.ParseAddr(*nc.IPAddress)
	if err != nil {
		return err
	}
	if !ipAddr.Is4() {
		return fmt.Errorf("ip address %s is not an IPv4 address", *nc.IPAddress)
	}

	seenKey := desiredIPSeenKey(networkName, ipAddr.String())
	if _, ok := seenDesiredIPs[seenKey]; ok {
		return fmt.Errorf("duplicate desired ip %s on network %s", ipAddr, networkName)
	}
	seenDesiredIPs[seenKey] = struct{}{}

	poolInfo, err := util.LoadPool(ipPool)
	if err != nil {
		return err
	}

	if !poolInfo.IPNet.Contains(ipAddr.AsSlice()) {
		return fmt.Errorf("ip address %s is not within subnet %s", ipAddr, ipPool.Spec.IPv4Config.CIDR)
	}
	if poolInfo.StartIPAddr.IsValid() && ipAddr.Compare(poolInfo.StartIPAddr) < 0 ||
		poolInfo.EndIPAddr.IsValid() && ipAddr.Compare(poolInfo.EndIPAddr) > 0 {
		return fmt.Errorf("ip address %s is not within pool range %s-%s", ipAddr, poolInfo.StartIPAddr, poolInfo.EndIPAddr)
	}
	if ipAddr == poolInfo.NetworkIPAddr {
		return fmt.Errorf("ip address %s is the same as network ip", ipAddr)
	}
	if ipAddr == poolInfo.BroadcastIPAddr {
		return fmt.Errorf("ip address %s is the same as broadcast ip", ipAddr)
	}

	if ipPool.Status.IPv4 == nil {
		return nil
	}
	macAddress, ok := ipPool.Status.IPv4.Allocated[ipAddr.String()]
	if !ok {
		return nil
	}
	switch macAddress {
	case util.ExcludedMark, util.ReservedMark:
		return fmt.Errorf("ip address %s is not allocatable", ipAddr)
	default:
		if macAddress != nc.MACAddress {
			return fmt.Errorf("ip address %s is already allocated to mac %s", ipAddr, macAddress)
		}
	}

	return nil
}
