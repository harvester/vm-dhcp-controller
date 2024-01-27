package ippool

import (
	"fmt"
	"net/netip"
	"reflect"

	"github.com/harvester/webhook/pkg/server/admission"
	"github.com/sirupsen/logrus"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"

	networkv1 "github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/harvester/vm-dhcp-controller/pkg/util"
	"github.com/harvester/vm-dhcp-controller/pkg/webhook"
)

const (
	Start EndpointType = "start"
	End   EndpointType = "end"
)

type EndpointType string

type Mutator struct {
	admission.DefaultMutator
}

func NewMutator() *Mutator {
	return &Mutator{}
}

func (m *Mutator) Create(_ *admission.Request, newObj runtime.Object) (admission.Patch, error) {
	ipPool := newObj.(*networkv1.IPPool)

	serverIP, err := ensureServerIP(
		ipPool.Spec.IPv4Config.ServerIP,
		ipPool.Spec.IPv4Config.CIDR,
		ipPool.Spec.IPv4Config.Router,
		ipPool.Spec.IPv4Config.Pool.Exclude,
	)
	if err != nil {
		return nil, fmt.Errorf(webhook.CreateErr, "IPPool", ipPool.Namespace, ipPool.Name, err)
	}

	pool, err := ensurePoolRange(
		ipPool.Spec.IPv4Config.Pool,
		ipPool.Spec.IPv4Config.CIDR,
	)
	if err != nil {
		return nil, fmt.Errorf(webhook.CreateErr, "IPPool", ipPool.Namespace, ipPool.Name, err)
	}

	var patch admission.Patch
	if pool != nil {
		patch = append(patch, admission.Patch{
			{
				Op:    admission.PatchOpReplace,
				Path:  "/spec/ipv4Config/pool",
				Value: *pool,
			},
		}...)
	}
	if serverIP != nil {
		patch = append(patch, admission.Patch{
			{
				Op:    admission.PatchOpReplace,
				Path:  "/spec/ipv4Config/serverIP",
				Value: *serverIP,
			},
		}...)
	}

	return patch, nil
}

func (m *Mutator) Resource() admission.Resource {
	return admission.Resource{
		Names:      []string{"ippools"},
		Scope:      admissionregv1.NamespacedScope,
		APIGroup:   networkv1.SchemeGroupVersion.Group,
		APIVersion: networkv1.SchemeGroupVersion.Version,
		ObjectType: &networkv1.IPPool{},
		OperationTypes: []admissionregv1.OperationType{
			admissionregv1.Create,
		},
	}
}

func ensureServerIP(server string, cidr, router string, excludes []string) (*string, error) {
	var maskedIPAddrList []netip.Addr

	ipNet, networkIPAddr, broadcastIPAddr, err := util.LoadCIDR(cidr)
	if err != nil {
		return nil, err
	}

	routerIPAddr, err := netip.ParseAddr(router)
	if err == nil {
		maskedIPAddrList = append(maskedIPAddrList, routerIPAddr)
	}

	serverIPAddr, err := netip.ParseAddr(server)
	if err != nil {
		serverIPAddr = netip.Addr{}
	}

	for _, exclude := range excludes {
		var excludeIPAddr netip.Addr
		excludeIPAddr, err = netip.ParseAddr(exclude)
		if err != nil {
			return nil, err
		}
		maskedIPAddrList = append(maskedIPAddrList, excludeIPAddr)
	}

	if !serverIPAddr.IsValid() {
		for serverIPAddr = networkIPAddr.Next(); ipNet.Contains(serverIPAddr.AsSlice()); serverIPAddr = serverIPAddr.Next() {
			if util.IsIPAddrInList(serverIPAddr, maskedIPAddrList) {
				continue
			}

			if serverIPAddr.As4() == broadcastIPAddr.As4() {
				break
			}

			serverIPStr := serverIPAddr.String()
			logrus.Infof("auto assign serverIP=%s", serverIPStr)

			return &serverIPStr, nil
		}

		return nil, fmt.Errorf("fail to assign ip for dhcp server")
	}

	return nil, nil
}

func ensurePoolRange(pool networkv1.Pool, cidr string) (*networkv1.Pool, error) {
	startIPAddr, err := netip.ParseAddr(pool.Start)
	if err != nil {
		startIPAddr = netip.Addr{}
	}

	endIPAddr, err := netip.ParseAddr(pool.End)
	if err != nil {
		endIPAddr = netip.Addr{}
	}

	ipNet, networkIPAddr, broadcastIPAddr, err := util.LoadCIDR(cidr)
	if err != nil {
		return nil, err
	}

	newPool := pool

	if !startIPAddr.IsValid() {
		startIPAddr = networkIPAddr.Next()

		if !ipNet.Contains(startIPAddr.AsSlice()) {
			logrus.Warningf("start ip is out of subnet")
		}

		newPool.Start = startIPAddr.String()
	}

	if !endIPAddr.IsValid() {
		endIPAddr = broadcastIPAddr.Prev()

		if !ipNet.Contains(endIPAddr.AsSlice()) {
			logrus.Warningf("end ip is out of subnet")
		}

		newPool.End = endIPAddr.String()
	}

	if startIPAddr.Compare(endIPAddr) > 0 {
		return nil, fmt.Errorf("invalid pool range")
	}

	if !reflect.DeepEqual(newPool, pool) {
		logrus.Infof("auto assign startIP=%s, endIP=%s", startIPAddr.String(), endIPAddr.String())
		return &newPool, nil
	}

	return nil, nil
}
