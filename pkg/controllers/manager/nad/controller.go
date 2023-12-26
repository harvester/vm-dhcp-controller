package nad

import (
	"context"

	cniv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"k8s.io/klog/v2"

	"github.com/starbops/vm-dhcp-controller/pkg/config"
	ctlcniv1 "github.com/starbops/vm-dhcp-controller/pkg/generated/controllers/k8s.cni.cncf.io/v1"
	ctlnetworkv1 "github.com/starbops/vm-dhcp-controller/pkg/generated/controllers/network.harvesterhci.io/v1alpha1"
)

const controllerName = "vm-dhcp-nad-controller"

type Handler struct {
	nadClient    ctlcniv1.NetworkAttachmentDefinitionClient
	nadCache     ctlcniv1.NetworkAttachmentDefinitionCache
	ippoolClient ctlnetworkv1.IPPoolClient
	ippoolCache  ctlnetworkv1.IPPoolCache
}

func Register(ctx context.Context, management *config.Management) error {
	ippools := management.HarvesterNetworkFactory.Network().V1alpha1().IPPool()
	nads := management.CniFactory.K8s().V1().NetworkAttachmentDefinition()

	handler := &Handler{
		nadClient:    nads,
		nadCache:     nads.Cache(),
		ippoolClient: ippools,
		ippoolCache:  ippools.Cache(),
	}

	nads.OnChange(ctx, controllerName, handler.OnChange)
	nads.OnRemove(ctx, controllerName, handler.OnRemove)

	return nil
}

func (h *Handler) OnChange(key string, nad *cniv1.NetworkAttachmentDefinition) (*cniv1.NetworkAttachmentDefinition, error) {
	if nad == nil || nad.DeletionTimestamp != nil {
		return nil, nil
	}

	klog.Infof("nad configuration %s/%s has been changed: %s", nad.Namespace, nad.Name, nad.Spec.Config)

	return nad, nil
}

func (h *Handler) OnRemove(key string, nad *cniv1.NetworkAttachmentDefinition) (*cniv1.NetworkAttachmentDefinition, error) {
	if nad == nil {
		return nil, nil
	}

	klog.Infof("nad configuration %s/%s has been removed", nad.Namespace, nad.Name)

	return nad, nil
}
