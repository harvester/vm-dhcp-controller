package node

import (
	"context"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/pointer"

	"github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io"
	"github.com/harvester/vm-dhcp-controller/pkg/config"
	ctlcorev1 "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/core/v1"
)

const (
	controllerName           = "vm-dhcp-node-controller"
	vmDHCPControllerLabelKey = network.GroupName + "/vm-dhcp-controller"
)

type Handler struct {
	agentNamespace string

	podClient ctlcorev1.PodClient
	podCache  ctlcorev1.PodCache
}

func Register(ctx context.Context, management *config.Management) error {
	nodes := management.CoreFactory.Core().V1().Node()
	pods := management.CoreFactory.Core().V1().Pod()

	handler := &Handler{
		agentNamespace: management.Options.AgentNamespace,
		podClient:      pods,
		podCache:       pods.Cache(),
	}

	nodes.OnChange(ctx, controllerName, handler.OnChange)

	return nil
}

func (h *Handler) OnChange(key string, node *corev1.Node) (*corev1.Node, error) {
	if node == nil {
		return nil, nil
	}

	if isNodeReady(node) {
		return node, nil
	}

	logrus.Debugf("(node.OnChange) node %s not ready, deleting agent pods", node.Name)

	selector := labels.Set{vmDHCPControllerLabelKey: "agent"}.AsSelector()
	pods, err := h.podCache.List(h.agentNamespace, selector)
	if err != nil {
		return node, err
	}

	for _, pod := range pods {
		if pod.Spec.NodeName != node.Name {
			continue
		}
		if pod.DeletionTimestamp != nil {
			continue
		}
		logrus.Infof("(node.OnChange) deleting agent pod %s/%s on node %s", pod.Namespace, pod.Name, node.Name)
		opts := metav1.DeleteOptions{GracePeriodSeconds: pointer.Int64(0)}
		if err := h.podClient.Delete(pod.Namespace, pod.Name, &opts); err != nil && !apierrors.IsNotFound(err) {
			return node, err
		}
	}

	return node, nil
}

func isNodeReady(node *corev1.Node) bool {
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}
