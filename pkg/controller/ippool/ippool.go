package ippool

import (
	"context"
	"fmt"
	"net"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kihv1 "github.com/joeyloman/kubevirt-ip-helper/pkg/apis/kubevirtiphelper.k8s.binbash.org/v1"
	"github.com/joeyloman/kubevirt-ip-helper/pkg/util"

	log "github.com/sirupsen/logrus"
)

func (c *Controller) registerIPPool(pool *kihv1.IPPool) (err error) {
	log.Infof("(ippool.registerIPPool) [%s] new IPPool added", pool.Name)

	// DEBUG
	// ifaces, err := util.ListInterfaces()
	// if err != nil {
	// 	return
	// }
	// log.Debugf("(ippool.registerIPPool) network interfaces: %+v", ifaces)
	// end DEBUG

	nic, err := util.GetNicFromIp(net.ParseIP(pool.Spec.IPv4Config.ServerIP))
	if err != nil {
		return
	}

	log.Debugf("(ippool.registerIPPool) [%s] nic found: [%s]", pool.Name, nic)

	// start a dhcp service thread if the serverip is bound to a nic
	if nic != "" {
		c.dhcp.Run(nic, pool.Spec.IPv4Config.ServerIP)
	}

	if err = c.ipam.NewSubnet(
		pool.Spec.NetworkName,
		pool.Spec.IPv4Config.Subnet,
		pool.Spec.IPv4Config.Pool.Start,
		pool.Spec.IPv4Config.Pool.End,
	); err != nil {
		return
	}

	// mark the exclude ips as used
	for _, v := range pool.Spec.IPv4Config.Pool.Exclude {
		ip, err := c.ipam.GetIP(pool.Spec.NetworkName, v)
		if err != nil {
			return fmt.Errorf("(ippool.registerIPPool) [%s] ipam error while excluding ip [%s]: %s",
				pool.Name, v, err)
		}

		// maybe unnecesarry check, but just to make sure
		if ip != v {
			return fmt.Errorf("(ippool.registerIPPool) [%s] got ip [%s] from ipam, but it doesn't match the exclude ip [%s]",
				pool.Name, ip, v)
		}
	}

	// rebuild the pool status after restarting the process
	rPool, err := c.resetIPPoolStatus(pool)
	if err != nil {
		return
	}

	// cache the pool with an empty status
	if err = c.cache.Add(rPool); err != nil {
		return
	}

	return
}

func (c *Controller) cleanupIPPoolObjects(pool kihv1.IPPool) (err error) {
	log.Debugf("(ippool.cleanupIPPoolObjects) [%s] starting cleanup of IPPool", pool.Name)

	nic, err := util.GetNicFromIp(net.ParseIP(pool.Spec.IPv4Config.ServerIP))
	if err != nil {
		return
	}

	// TODO: remove
	//log.Debugf("(ippool.cleanupIPPoolObjects) [%s] nic found: [%s]", pool.Name, nic)

	if nic != "" {
		err := c.dhcp.Stop(nic)
		if err != nil {
			log.Errorf("(ippool.cleanupIPPoolObjects) [%s] error while stopping DHCP service on nic %s",
				pool.Name, err.Error())
		}
	}

	c.ipam.DeleteSubnet(pool.Spec.NetworkName)
	c.cache.Delete("pool", pool.Spec.NetworkName)

	return
}

func (c *Controller) resetIPPoolStatus(pool *kihv1.IPPool) (uPool *kihv1.IPPool, err error) {
	cPool, err := c.kihClientset.KubevirtiphelperV1().IPPools().Get(context.TODO(), pool.Name, metav1.GetOptions{})
	if err != nil {
		return uPool, fmt.Errorf("(ippool.resetIPPoolStatus) [%s] cannot get IPPool: %s", pool.Name, err.Error())
	}

	// if the timestamp is not set, set it to the current local time
	if cPool.Status.LastUpdate.IsZero() {
		cPool.Status.LastUpdateBeforeStart = metav1.Now()
	} else {
		// save the last status update to handle the vmnetcfg objects when the program is (re)started
		cPool.Status.LastUpdateBeforeStart = cPool.Status.LastUpdate
	}

	cPool.Status.LastUpdate = metav1.Now()

	allocatedExcludes := make(map[string]string)
	for _, v := range pool.Spec.IPv4Config.Pool.Exclude {
		// TODO: remove
		// log.Debugf("(ippool.resetIPPoolStatus) [%s] adding exclude ip [%s] to the status for network [%s]", pool.Name, v, pool.Spec.NetworkName)

		allocatedExcludes[v] = "EXCLUDED"
	}
	cPool.Status.IPv4.Allocated = allocatedExcludes
	cPool.Status.IPv4.Used = c.ipam.Used(pool.Spec.NetworkName)
	cPool.Status.IPv4.Available = c.ipam.Available(pool.Spec.NetworkName)

	uPool, err = c.kihClientset.KubevirtiphelperV1().IPPools().UpdateStatus(context.TODO(), cPool, metav1.UpdateOptions{})
	if err != nil {
		return uPool, fmt.Errorf("(ippool.resetIPPoolStatus) [%s] cannot update status of IPPool: %s",
			cPool.Name, err.Error())
	}

	return
}
