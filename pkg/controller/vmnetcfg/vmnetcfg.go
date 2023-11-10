package vmnetcfg

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kihv1 "github.com/joeyloman/kubevirt-ip-helper/pkg/apis/kubevirtiphelper.k8s.binbash.org/v1"

	log "github.com/sirupsen/logrus"
)

func (c *Controller) updateVirtualMachineNetworkConfig(eventAction string, vmnetcfg *kihv1.VirtualMachineNetworkConfig) (err error) {
	var networkChange bool = false
	var skipNic bool = false

	log.Tracef("(vmnetcfg.updateVirtualMachineNetworkConfig) [%s/%s] processing new vmnetcfg [%+v]",
		vmnetcfg.Namespace, vmnetcfg.Name, vmnetcfg)

	// cleanup the network configuration if the object is marked for deletion
	if vmnetcfg.ObjectMeta.DeletionTimestamp != nil {
		if err := c.cleanupVirtualMachineNetworkConfig(vmnetcfg); err != nil {
			return fmt.Errorf("(vmnetcfg.updateVirtualMachineNetworkConfig) [%s/%s] failed to cleanup vmnetcfg: %s",
				vmnetcfg.Namespace, vmnetcfg.Name, err.Error())
		}

		return
	}

	newVmNetCfg := vmnetcfg.DeepCopy()
	newVmNetCfgs := []kihv1.NetworkConfig{}
	newNetCfgStatusList := []kihv1.NetworkConfigStatus{}
	for _, v := range vmnetcfg.Spec.NetworkConfig {
		// TODO: remove
		// log.Debugf("(vmnetcfg.updateVirtualMachineNetworkConfig) [%s/%s] processing hwaddr [%s] and network [%s]",
		// 	vmnetcfg.Namespace, vmnetcfg.Name, v.MACAddress, v.NetworkName)

		pool, err := c.cache.Get("pool", v.NetworkName)
		if err != nil {
			return err
		}

		// create a fresh nic status
		netcfgStatus := kihv1.NetworkConfigStatus{}
		netcfgStatus.MACAddress = v.MACAddress
		netcfgStatus.NetworkName = v.NetworkName

		// skip the network interface updates when it has a status ERROR
		skipNic = false
		for _, nic := range vmnetcfg.Status.NetworkConfig {
			if v.MACAddress == nic.MACAddress && v.NetworkName == nic.NetworkName && nic.Status == "ERROR" {
				netcfgStatus.Status = nic.Status
				netcfgStatus.Message = nic.Message
				newNetCfgStatusList = append(newNetCfgStatusList, netcfgStatus)

				skipNic = true

				break
			}
		}

		// check for duplicate mac address registrations
		if !skipNic && c.dhcp.CheckLease(v.MACAddress) {
			lease := c.dhcp.GetLease(v.MACAddress)
			if lease.Reference != fmt.Sprintf("%s/%s", vmnetcfg.Namespace, vmnetcfg.Spec.VMName) {
				log.Errorf("(vmnetcfg.updateVirtualMachineNetworkConfig) [%s/%s] hwaddr %s belongs to %s",
					vmnetcfg.Namespace, vmnetcfg.Name, v.MACAddress, lease.Reference)

				netcfgStatus.Status = "ERROR"
				netcfgStatus.Message = "macaddress belongs to another vm"
				newNetCfgStatusList = append(newNetCfgStatusList, netcfgStatus)

				skipNic = true
			}
		}

		// check the added vmnetcfgs which are new and don't have a networkconfig status
		if !skipNic && eventAction == ADD && len(vmnetcfg.Status.NetworkConfig) == 0 {
			log.Tracef("(vmnetcfg.updateVirtualMachineNetworkConfig) [%s/%s] vmnetcfg.CreateTimestamp=%s, pool.LastUpdateBeforeStart=%s, pool.lastUpdate=%s",
				vmnetcfg.Namespace, vmnetcfg.Name, vmnetcfg.CreationTimestamp,
				pool.(kihv1.IPPool).Status.LastUpdateBeforeStart.Time,
				pool.(kihv1.IPPool).Status.LastUpdate.Time)

			// put the network interfaces in ERROR state when the vmnetcfg is (manually) created between
			// the last status update before the program was stopped and the restart of the program.
			// this could cause a possible hijack of ip addresses which are already registered in existing vmnetcfgs.
			// this should be automatically handled by the vm controller and not manually when the program is not running.
			if vmnetcfg.CreationTimestamp.After(pool.(kihv1.IPPool).Status.LastUpdateBeforeStart.Time) &&
				pool.(kihv1.IPPool).Status.LastUpdate.After(vmnetcfg.CreationTimestamp.Time) {
				log.Errorf("(vmnetcfg.updateVirtualMachineNetworkConfig) [%s/%s] vmnetcfg was manually created after this program was (re)started, preventing possible ip hijack",
					vmnetcfg.Namespace, vmnetcfg.Name)

				netcfgStatus.Status = "ERROR"
				netcfgStatus.Message = "vmnetcfg was manually created after this program was (re)started, preventing possible ip hijack"
				newNetCfgStatusList = append(newNetCfgStatusList, netcfgStatus)

				skipNic = true
			}
		}

		if skipNic {
			log.Errorf("(vmnetcfg.updateVirtualMachineNetworkConfig) [%s/%s] network interface has an error status, skipping updates",
				vmnetcfg.Namespace, vmnetcfg.Name)

			newVmNetCfgs = append(newVmNetCfgs, v)

			continue
		}

		// handle ip changes in the vmnetcfg object
		if c.dhcp.CheckLease(v.MACAddress) {
			lease := c.dhcp.GetLease(v.MACAddress)
			if lease.ClientIP.String() != v.IPAddress {
				log.Warnf("(vmnetcfg.updateVirtualMachineNetworkConfig) [%s/%s] ip address update found for hwaddr=%s, oldip=%s, newip=%s, starting cleanup of old ip address",
					vmnetcfg.Namespace, vmnetcfg.Name, v.MACAddress, lease.ClientIP.String(), v.IPAddress)

				oldNetcfg := kihv1.NetworkConfig{}
				oldNetcfg.NetworkName = v.NetworkName
				oldNetcfg.MACAddress = v.MACAddress
				oldNetcfg.IPAddress = lease.ClientIP.String()
				c.cleanupNetworkInterface(vmnetcfg, &oldNetcfg)
			} else {
				log.Debugf("(vmnetcfg.updateVirtualMachineNetworkConfig) [%s/%s] hwaddr %s already exists in the leases, skipping interface",
					vmnetcfg.Namespace, vmnetcfg.Name, v.MACAddress)

				newVmNetCfgs = append(newVmNetCfgs, v)

				// set the old status
				for _, nic := range vmnetcfg.Status.NetworkConfig {
					if v.MACAddress == nic.MACAddress && v.NetworkName == nic.NetworkName {
						netcfgStatus.Status = nic.Status
						netcfgStatus.Message = nic.Message
						newNetCfgStatusList = append(newNetCfgStatusList, netcfgStatus)

						break
					}
				}

				continue
			}
		}

		// if v.IPAddress is not empty we register it else we get a new one
		ip, err := c.ipam.GetIP(v.NetworkName, v.IPAddress)
		if err != nil {
			log.Errorf("(vmnetcfg.updateVirtualMachineNetworkConfig) [%s/%s] ipam error: %s, skipping interface",
				vmnetcfg.Namespace, vmnetcfg.Name, err)

			newVmNetCfgs = append(newVmNetCfgs, v)

			netcfgStatus.Status = "ERROR"
			netcfgStatus.Message = err.Error()
			newNetCfgStatusList = append(newNetCfgStatusList, netcfgStatus)

			continue
		}
		log.Tracef("(vmnetcfg.updateVirtualMachineNetworkConfig) [%s/%s] got IP %s from ipam", vmnetcfg.Namespace, vmnetcfg.Name, ip)

		ipnet, err := netip.ParsePrefix(pool.(kihv1.IPPool).Spec.IPv4Config.Subnet)
		if err != nil {
			if ipamErr := c.ipam.ReleaseIP(v.NetworkName, v.IPAddress); ipamErr != nil {
				log.Errorf("(vmnetcfg.updateVirtualMachineNetworkConfig) [%s/%s] %s",
					vmnetcfg.Namespace, vmnetcfg.Name, ipamErr)
			}

			// abort update
			return err
		}
		subnetMask := net.CIDRMask(ipnet.Bits(), 32)
		ref := fmt.Sprintf("%s/%s", vmnetcfg.Namespace, vmnetcfg.Spec.VMName)

		c.dhcp.AddLease(
			v.MACAddress,
			pool.(kihv1.IPPool).Spec.IPv4Config.ServerIP,
			ip,
			net.IP(subnetMask).String(),
			pool.(kihv1.IPPool).Spec.IPv4Config.Router,
			pool.(kihv1.IPPool).Spec.IPv4Config.DNS,
			pool.(kihv1.IPPool).Spec.IPv4Config.DomainName,
			pool.(kihv1.IPPool).Spec.IPv4Config.DomainSearch,
			pool.(kihv1.IPPool).Spec.IPv4Config.NTP,
			pool.(kihv1.IPPool).Spec.IPv4Config.LeaseTime,
			ref,
		)

		n := kihv1.NetworkConfig{}
		n.IPAddress = ip
		n.MACAddress = v.MACAddress
		n.NetworkName = v.NetworkName
		newVmNetCfgs = append(newVmNetCfgs, n)

		netcfgStatus.Status = "OK"
		netcfgStatus.Message = "IP address successfully allocated"
		newNetCfgStatusList = append(newNetCfgStatusList, netcfgStatus)

		if err := c.updateIPPoolStatus(
			ADD,
			vmnetcfg.Namespace,
			vmnetcfg.Spec.VMName,
			ip,
			v.NetworkName,
			v.MACAddress,
			pool.(kihv1.IPPool).Name,
		); err != nil {
			log.Errorf("(vmnetcfg.updateVirtualMachineNetworkConfig) [%s/%s] %s",
				vmnetcfg.Namespace, vmnetcfg.Name, err)
		}

		if err := c.updateIPPoolMetrics(pool.(kihv1.IPPool).Name); err != nil {
			log.Errorf("(vmnetcfg.updateVirtualMachineNetworkConfig) [%s/%s] %s",
				vmnetcfg.Namespace, vmnetcfg.Name, err)
		}

		networkChange = true
	}

	newVmnetCfgStatus := kihv1.VirtualMachineNetworkConfigStatus{}
	newVmnetCfgStatus.NetworkConfig = newNetCfgStatusList

	if !networkChange {
		log.Debugf("(vmnetcfg.updateVirtualMachineNetworkConfig) [%s/%s] no network changes detected, skipping object update",
			vmnetcfg.Namespace, vmnetcfg.Name)

		// only update the status and metrics when the status.networkconfig array has items
		if len(newVmnetCfgStatus.NetworkConfig) > 0 {
			if err := c.updateVirtualMachineNetworkConfigStatus(vmnetcfg, &newVmnetCfgStatus); err != nil {
				log.Errorf("(vmnetcfg.updateVirtualMachineNetworkConfig) [%s/%s] %s",
					vmnetcfg.ObjectMeta.Namespace, vmnetcfg.ObjectMeta.Name, err)
			}

			if err := c.updateVirtualMachineNetworkConfigMetrics(vmnetcfg.Namespace, vmnetcfg.Name); err != nil {
				log.Errorf("(vmnetcfg.updateVirtualMachineNetworkConfig) [%s/%s] %s",
					vmnetcfg.Namespace, vmnetcfg.Name, err)
			}
		}

		return
	}

	newVmNetCfg.Spec.NetworkConfig = newVmNetCfgs

	log.Tracef("(vmnetcfg.updateVirtualMachineNetworkConfig) [%s/%s] updating vmnetcfg object to [%+v]",
		vmnetcfg.Namespace, vmnetcfg.Name, newVmNetCfg)

	vmNetCfgObj, err := c.kihClientset.KubevirtiphelperV1().VirtualMachineNetworkConfigs(newVmNetCfg.Namespace).Update(context.TODO(), newVmNetCfg, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("(vmnetcfg.updateVirtualMachineNetworkConfig) [%s/%s] cannot update VirtualMachineNetworkConfig object: %s",
			vmNetCfgObj.ObjectMeta.Namespace, vmNetCfgObj.ObjectMeta.Name, err.Error())
	}

	log.Debugf("(vmnetcfg.updateVirtualMachineNetworkConfig) [%s/%s] successfully processed the network configuration",
		vmNetCfgObj.ObjectMeta.Namespace, vmNetCfgObj.ObjectMeta.Name)

	if err := c.updateVirtualMachineNetworkConfigStatus(vmNetCfgObj, &newVmnetCfgStatus); err != nil {
		log.Errorf("(vmnetcfg.updateVirtualMachineNetworkConfig) [%s/%s] %s",
			vmNetCfgObj.ObjectMeta.Namespace, vmNetCfgObj.ObjectMeta.Name, err)
	}

	if err := c.updateVirtualMachineNetworkConfigMetrics(vmNetCfgObj.Namespace, vmNetCfgObj.Name); err != nil {
		log.Errorf("(vmnetcfg.updateVirtualMachineNetworkConfig) [%s/%s] %s",
			vmNetCfgObj.Namespace, vmNetCfgObj.Name, err)
	}

	return
}

func (c *Controller) cleanupNetworkInterface(vmnetcfg *kihv1.VirtualMachineNetworkConfig, netCfg *kihv1.NetworkConfig) {
	log.Debugf("(vmnetcfg.cleanupNetworkInterface) [%s/%s] cleaning interface with hwaddr=%s, networkname=%s, ipaddress=%s",
		vmnetcfg.Namespace, vmnetcfg.Name, netCfg.MACAddress, netCfg.NetworkName, netCfg.IPAddress)

	if err := c.dhcp.DeleteLease(netCfg.MACAddress); err != nil {
		log.Errorf("(vmnetcfg.cleanupNetworkInterface) [%s/%s] error deleting lease from dhcp: %s",
			vmnetcfg.Namespace, vmnetcfg.Name, err)
	}

	if err := c.ipam.ReleaseIP(netCfg.NetworkName, netCfg.IPAddress); err != nil {
		log.Errorf("(vmnetcfg.cleanupNetworkInterface) [%s/%s] error releasing ip from ipam: %s",
			vmnetcfg.Namespace, vmnetcfg.Name, err)
	}

	pool, err := c.cache.Get("pool", netCfg.NetworkName)
	if err != nil {
		log.Errorf("(vmnetcfg.cleanupNetworkInterface) [%s/%s] %s",
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
			log.Errorf("(vmnetcfg.cleanupNetworkInterface) [%s/%s] %s",
				vmnetcfg.Namespace, vmnetcfg.Name, err)
		}

		if err := c.updateIPPoolMetrics(pool.(kihv1.IPPool).Name); err != nil {
			log.Errorf("(vmnetcfg.updateVirtualMachineNetworkConfig) [%s/%s] %s",
				vmnetcfg.Namespace, vmnetcfg.Name, err)
		}
	}
}

func (c *Controller) cleanupVirtualMachineNetworkConfig(vmnetcfg *kihv1.VirtualMachineNetworkConfig) (err error) {
	log.Debugf("(vmnetcfg.cleanupVirtualMachineNetworkConfig) [%s/%s] starting cleanup for vmnetcfg",
		vmnetcfg.Namespace, vmnetcfg.Name)

	for i := 0; i < len(vmnetcfg.Spec.NetworkConfig); i++ {
		c.cleanupNetworkInterface(vmnetcfg, &vmnetcfg.Spec.NetworkConfig[i])
	}

	c.deleteVirtualMachineNetworkConfigMetrics(vmnetcfg)

	updatedVmNetCfg := vmnetcfg.DeepCopy()
	newFinalizers := []string{}
	for i := 0; i < len(vmnetcfg.ObjectMeta.Finalizers); i++ {
		if vmnetcfg.ObjectMeta.Finalizers[i] != "kubevirtiphelper" {
			// TODO: remove
			// log.Debugf("(vmnetcfg.cleanupVirtualMachineNetworkConfig) [%s/%s] adding finalizer %s",
			// 	vmnetcfg.Namespace, vmnetcfg.Name, vmnetcfg.ObjectMeta.Finalizers[i])
			newFinalizers = append(newFinalizers, vmnetcfg.ObjectMeta.Finalizers[i])
		}
	}

	if len(newFinalizers) == len(updatedVmNetCfg.ObjectMeta.Finalizers) {
		// TODO: remove
		// log.Warnf("(vmnetcfg.cleanupVirtualMachineNetworkConfig) [%s/%s] no finalizers found for VirtualMachineNetworkConfig object",
		// 	vmnetcfg.Namespace, vmnetcfg.Name)

		return
	}

	updatedVmNetCfg.ObjectMeta.Finalizers = newFinalizers
	vmNetCfgObj, err := c.kihClientset.KubevirtiphelperV1().VirtualMachineNetworkConfigs(updatedVmNetCfg.Namespace).Update(context.TODO(), updatedVmNetCfg, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("(vmnetcfg.cleanupVirtualMachineNetworkConfig) [%s/%s] cannot remove finalizers for VirtualMachineNetworkConfig object: %s",
			vmNetCfgObj.Namespace, vmNetCfgObj.Name, err.Error())
	}

	log.Debugf("(vmnetcfg.cleanupVirtualMachineNetworkConfig) [%s/%s] succesfully removed finalizers for VirtualMachineNetworkConfig object",
		vmNetCfgObj.Namespace, vmNetCfgObj.Name)

	return
}

func (c *Controller) updateIPPoolStatus(event string, vmnetcfgNamespace string, vmnetcfgVMName string, ip string, networkName string, hwAddr string, poolName string) (err error) {
	currentPool, err := c.kihClientset.KubevirtiphelperV1().IPPools().Get(context.TODO(), poolName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("(vmnetcfg.updateIPPoolStatus) [%s/%s] cannot get IPPool %s: %s",
			vmnetcfgNamespace, vmnetcfgVMName, poolName, err.Error())
	}

	updatedPool := currentPool.DeepCopy()
	updatedAllocated := make(map[string]string)
	switch event {
	case ADD:
		for k, v := range currentPool.Status.IPv4.Allocated {
			if k == ip {
				return fmt.Errorf("(vmnetcfg.updateIPPoolStatus) [%s/%s] ip %s already found in IPPool status",
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
		return fmt.Errorf("(vmnetcfg.updateIPPoolStatus) [%s/%s] cannot update status of IPPool %s: %s",
			vmnetcfgNamespace, vmnetcfgVMName, updatedPool.Name, err.Error())
	}

	return
}

func (c *Controller) updateVirtualMachineNetworkConfigStatus(vmnetcfg *kihv1.VirtualMachineNetworkConfig, vmnetcfgStatus *kihv1.VirtualMachineNetworkConfigStatus) (err error) {
	vmnetcfg.Status = *vmnetcfgStatus

	vmNetCfgStatusObj, err := c.kihClientset.KubevirtiphelperV1().VirtualMachineNetworkConfigs(vmnetcfg.Namespace).UpdateStatus(context.TODO(), vmnetcfg, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("(vmnetcfg.updateVirtualMachineNetworkConfigStatus) [%s/%s] cannot update status of VirtualMachineNetworkConfig: %s",
			vmnetcfg.Namespace, vmnetcfg.Name, err.Error())
	}

	log.Debugf("(vmnetcfg.updateVirtualMachineNetworkConfigStatus) [%s/%s] successfully updated status of vmnetcfg object",
		vmNetCfgStatusObj.Namespace, vmNetCfgStatusObj.Name)

	return
}

func (c *Controller) updateIPPoolMetrics(poolName string) (err error) {
	pool, err := c.kihClientset.KubevirtiphelperV1().IPPools().Get(context.TODO(), poolName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("(vmnetcfg.updateIPPoolMetrics) cannot get IPPool %s: %s", poolName, err.Error())
	}

	c.metrics.UpdateIPPoolUsed(pool.Name, pool.Spec.IPv4Config.Subnet, pool.Spec.NetworkName, pool.Status.IPv4.Used)
	c.metrics.UpdateIPPoolAvailable(pool.Name, pool.Spec.IPv4Config.Subnet, pool.Spec.NetworkName, pool.Status.IPv4.Available)

	return
}

func (c *Controller) updateVirtualMachineNetworkConfigMetrics(vmnetcfgNamespace string, vmnetcfgName string) (err error) {
	vmnetcfg, err := c.kihClientset.KubevirtiphelperV1().VirtualMachineNetworkConfigs(vmnetcfgNamespace).Get(context.TODO(), vmnetcfgName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("(vmnetcfg.updateVirtualMachineNetworkConfigMetrics) cannot get VirtualMachineNetworkConfig %s/%s: %s",
			vmnetcfgNamespace, vmnetcfgName, err.Error())
	}

	for _, netstat := range vmnetcfg.Status.NetworkConfig {
		for _, netcfg := range vmnetcfg.Spec.NetworkConfig {
			if netstat.MACAddress == netcfg.MACAddress {
				c.metrics.UpdateVmNetCfgStatus(
					fmt.Sprintf("%s/%s", vmnetcfgNamespace, vmnetcfgName),
					netstat.NetworkName,
					netstat.MACAddress,
					netcfg.IPAddress,
					netstat.Status,
				)
			}
		}
	}

	return
}

func (c *Controller) deleteVirtualMachineNetworkConfigMetrics(vmnetcfg *kihv1.VirtualMachineNetworkConfig) {
	c.metrics.DeleteVmNetCfgStatus(fmt.Sprintf("%s/%s", vmnetcfg.Namespace, vmnetcfg.Name))
}
