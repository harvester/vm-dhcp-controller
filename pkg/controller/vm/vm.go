package vm

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	log "github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubevirtV1 "kubevirt.io/api/core/v1"

	kihv1 "github.com/joeyloman/kubevirt-ip-helper/pkg/apis/kubevirtiphelper.k8s.binbash.org/v1"
)

func (c *Controller) handleVirtualMachineObjectChange(vm *kubevirtV1.VirtualMachine) (err error) {
	vmnetcfg, err := c.kihClientset.KubevirtiphelperV1().VirtualMachineNetworkConfigs(vm.Namespace).Get(context.TODO(), vm.Name, metav1.GetOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return c.createVirtualMachineNetworkConfigObject(vm)
		} else {
			return
		}
	}

	return c.updateVirtualMachineNetworkConfigObject(vm, vmnetcfg)
}

func (c *Controller) createVirtualMachineNetworkConfigObject(vm *kubevirtV1.VirtualMachine) (err error) {
	log.Tracef("(vm.createVirtualMachineNetworkConfigObject) [%s/%s] processing new VirtualMachine  [%+v]",
		vm.Namespace, vm.Name, vm)

	newVmNetCfg := kihv1.VirtualMachineNetworkConfig{}
	newVmNetCfg.ObjectMeta.Name = vm.ObjectMeta.Name
	newVmNetCfg.ObjectMeta.Namespace = vm.ObjectMeta.Namespace
	finalizers := []string{}
	finalizers = append(finalizers, "kubevirtiphelper")
	newVmNetCfg.ObjectMeta.Finalizers = finalizers
	newVmNetCfg.Spec.VMName = vm.ObjectMeta.Name

	netCfgs, err := c.getNetworkConfigs(vm, nil)
	if err != nil {
		return
	}
	if len(netCfgs) < 1 {
		log.Debugf("(vm.createVirtualMachineNetworkConfig) [%s/%s] no network configuration found for vm",
			vm.Namespace, vm.Name)

		return
	}
	newVmNetCfg.Spec.NetworkConfig = netCfgs

	vmNetCfgObj, err := c.kihClientset.KubevirtiphelperV1().VirtualMachineNetworkConfigs(newVmNetCfg.Namespace).Create(context.TODO(), &newVmNetCfg, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("(vm.createVirtualMachineNetworkConfig) [%s/%s] cannot create VirtualMachineNetworkConfig object for vm: %s",
			vm.Namespace, vm.Name, err.Error())
	}

	log.Debugf("(vm.createVirtualMachineNetworkConfig) [%s/%s] successfully created vmnetcfg object [%s/%s]",
		vm.Namespace, vm.Name, vmNetCfgObj.ObjectMeta.Namespace, vmNetCfgObj.ObjectMeta.Name)

	return
}

func (c *Controller) updateVirtualMachineNetworkConfigObject(vm *kubevirtV1.VirtualMachine, vmnetcfg *kihv1.VirtualMachineNetworkConfig) (err error) {
	log.Tracef("(vm.updateVirtualMachineNetworkConfigObject) [%s/%s] processing updated VirtualMachine  [%+v]",
		vm.Namespace, vm.Name, vm)

	newVmNetCfg := vmnetcfg.DeepCopy()

	netCfgs, err := c.getNetworkConfigs(vm, vmnetcfg.Spec.NetworkConfig)
	if err != nil {
		return
	}

	if reflect.DeepEqual(vmnetcfg.Spec.NetworkConfig, netCfgs) {
		log.Debugf("(vm.updateVirtualMachineNetworkConfigObject) [%s/%s] no network updates needed", vm.Namespace, vm.Name)
		return
	}

	newVmNetCfg.Spec.NetworkConfig = netCfgs

	log.Tracef("(vm.updateVirtualMachineNetworkConfigObject) [%s/%s] new vmnetcfg networkconfig: [%+v]",
		vm.Namespace, vm.Name, newVmNetCfg.Spec.NetworkConfig)

	// TODO: check the vmnetcfg status for ERROR(s) and skip them

	// when the nics in the vm differs from the vmnetcfg the mismatches should be cleaned up first
	var nicCleanup bool
	for _, curNetCfg := range vmnetcfg.Spec.NetworkConfig {
		nicCleanup = true
		for _, newNetCfg := range netCfgs {
			if curNetCfg.MACAddress == newNetCfg.MACAddress && curNetCfg.NetworkName == newNetCfg.NetworkName && curNetCfg.IPAddress == newNetCfg.IPAddress {
				nicCleanup = false
			}
		}
		if nicCleanup {
			c.cleanupNetworkInterface(vmnetcfg, &curNetCfg)
		}
	}

	vmNetCfgObj, err := c.kihClientset.KubevirtiphelperV1().VirtualMachineNetworkConfigs(newVmNetCfg.Namespace).Update(context.TODO(), newVmNetCfg, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("(vm.updateVirtualMachineNetworkConfigObject) [%s/%s] cannot update VirtualMachineNetworkConfig object for vm: %s",
			vm.Namespace, vm.Name, err.Error())
	}

	log.Debugf("(vm.updateVirtualMachineNetworkConfigObject) [%s/%s] successfully updated vmnetcfg object [%s/%s]",
		vm.Namespace, vm.Name, vmNetCfgObj.ObjectMeta.Namespace, vmNetCfgObj.ObjectMeta.Name)

	return
}

func (c *Controller) deleteVirtualMachineNetworkConfigObject(vmNamespace string, vmName string) (err error) {
	if !c.checkVirtualMachineNetworkConfigObject(vmNamespace, vmName) {
		log.Warnf("(vm.deleteVirtualMachineNetworkConfigObject) [%s/%s] vmnetcfg %s/%s does not exists",
			vmNamespace, vmName, vmNamespace, vmName)

		return
	}

	if err := c.kihClientset.KubevirtiphelperV1().VirtualMachineNetworkConfigs(vmNamespace).Delete(context.TODO(), vmName, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("(vm.deleteVirtualMachineNetworkConfigObject) [%s/%s] cannot delete VirtualMachineNetworkConfig object for vm: %s",
			vmNamespace, vmName, err.Error())
	}

	log.Debugf("(vm.deleteVirtualMachineNetworkConfigObject) [%s/%s] successfully released vmnetcfg object [%s/%s]",
		vmNamespace, vmName, vmNamespace, vmName)

	return
}

func (c *Controller) checkVirtualMachineNetworkConfigObject(vmNamespace string, vmName string) bool {
	if _, err := c.kihClientset.KubevirtiphelperV1().VirtualMachineNetworkConfigs(vmNamespace).Get(context.TODO(), vmName, metav1.GetOptions{}); err != nil {
		return false
	}

	return true
}

func (c *Controller) getNetworkConfigs(vm *kubevirtV1.VirtualMachine, curNetCfg []kihv1.NetworkConfig) (netCfgs []kihv1.NetworkConfig, err error) {
	for _, nic := range vm.Spec.Template.Spec.Domain.Devices.Interfaces {
		for _, net := range vm.Spec.Template.Spec.Networks {
			if nic.Name == net.Name {
				if net.Multus == nil {
					// we only support multus at the moment
					log.Warnf("(vm.getNetworkConfigs) [%s/%s] unsupported network type found!",
						vm.Namespace, vm.Name)
				} else if nic.MacAddress == "" {
					// when a new vm is created the macaddress doesn't exists immediately.
					// it takes a couple of object updates before the macaddress is assigned.
					// so avoid confusion and don't log errors here.
					log.Debugf("(vm.getNetworkConfigs) [%s/%s] no mac address found for vm",
						vm.Namespace, vm.Name)
				} else if net.Multus.NetworkName == "" {
					// the networkname should be there from the beginning
					log.Errorf("(vm.getNetworkConfigs) [%s/%s] no networkname found for vm",
						vm.Namespace, vm.Name)
				} else {
					if c.dhcp.CheckLease(nic.MacAddress) {
						lease := c.dhcp.GetLease(nic.MacAddress)
						if lease.Reference != fmt.Sprintf("%s/%s", vm.Namespace, vm.Name) {
							return netCfgs, fmt.Errorf("hwaddr %s belongs to %s instead of %s/%s, skipping vmnetcfg actions",
								nic.MacAddress, lease.Reference, vm.Namespace, vm.Name)
						}
					}

					netCfg := kihv1.NetworkConfig{}
					netCfg.MACAddress = nic.MacAddress
					netCfg.NetworkName = net.Multus.NetworkName

					for _, oldnet := range curNetCfg {
						if oldnet.MACAddress == nic.MacAddress && oldnet.NetworkName == net.Multus.NetworkName {
							netCfg.IPAddress = oldnet.IPAddress
						}
					}

					netCfgs = append(netCfgs, netCfg)
				}
			}
		}
	}

	return
}

func (c *Controller) cleanupNetworkInterface(vmnetcfg *kihv1.VirtualMachineNetworkConfig, netCfg *kihv1.NetworkConfig) {
	log.Debugf("(vm.cleanupNetworkInterface) [%s/%s] cleaning interface with hwaddr=%s, networkname=%s, ipaddress=%s",
		vmnetcfg.Namespace, vmnetcfg.Name, netCfg.MACAddress, netCfg.NetworkName, netCfg.IPAddress)

	if err := c.dhcp.DeleteLease(netCfg.MACAddress); err != nil {
		log.Errorf("(vm.cleanupNetworkInterface) [%s/%s] error deleting lease from dhcp: %s",
			vmnetcfg.Namespace, vmnetcfg.Name, err)
	}

	if err := c.ipam.ReleaseIP(netCfg.NetworkName, netCfg.IPAddress); err != nil {
		log.Errorf("(vm.cleanupNetworkInterface) [%s/%s] error releasing ip from ipam: %s",
			vmnetcfg.Namespace, vmnetcfg.Name, err)
	}

	pool, err := c.cache.Get("pool", netCfg.NetworkName)
	if err != nil {
		log.Errorf("(vm.cleanupNetworkInterface) [%s/%s] %s",
			vmnetcfg.Namespace, vmnetcfg.Name, err)
	} else {
		if err := c.updateIPPoolStatus(
			DELETE,
			vmnetcfg.Namespace,
			vmnetcfg.Spec.VMName,
			netCfg.IPAddress,
			netCfg.NetworkName,
			netCfg.MACAddress,
			pool.(kihv1.IPPool).Name,
		); err != nil {
			log.Errorf("(vm.cleanupNetworkInterface) [%s/%s] %s",
				vmnetcfg.Namespace, vmnetcfg.Name, err)
		}
	}
}

func (c *Controller) updateIPPoolStatus(event string, vmnetcfgNamespace string, vmnetcfgVMName string, ip string, networkName string, hwAddr string, poolName string) (err error) {
	currentPool, err := c.kihClientset.KubevirtiphelperV1().IPPools().Get(context.TODO(), poolName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("(vm.updateIPPoolStatus) [%s/%s] cannot get IPPool %s: %s",
			vmnetcfgNamespace, vmnetcfgVMName, poolName, err.Error())
	}

	updatedPool := currentPool.DeepCopy()
	updatedAllocated := make(map[string]string)
	switch event {
	case ADD:
		for k, v := range currentPool.Status.IPv4.Allocated {
			if k == ip {
				return fmt.Errorf("(vm.updateIPPoolStatus) [%s/%s] ip %s already found in IPPool status",
					vmnetcfgNamespace, vmnetcfgVMName, ip)
			}
			updatedAllocated[k] = v
		}
		updatedAllocated[ip] = fmt.Sprintf("%s/%s [%s]", vmnetcfgNamespace, vmnetcfgVMName, hwAddr)
	case DELETE:
		for k, v := range currentPool.Status.IPv4.Allocated {
			if k != ip {
				updatedAllocated[k] = v
			}
		}
	}
	updatedPool.Status.IPv4.Allocated = updatedAllocated
	updatedPool.Status.IPv4.Used = c.ipam.Used(networkName)
	updatedPool.Status.IPv4.Available = c.ipam.Available(networkName)
	updatedPool.Status.LastUpdate = metav1.Now()

	if _, err := c.kihClientset.KubevirtiphelperV1().IPPools().UpdateStatus(context.TODO(), updatedPool, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("(vm.updateIPPoolStatus) [%s/%s] cannot update status of IPPool %s: %s",
			vmnetcfgNamespace, vmnetcfgVMName, updatedPool.Name, err.Error())
	}

	return
}
