package util

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// CleanupTerminatingPods forcibly deletes pods with the specified component label
// that have been stuck in the Terminating state for longer than the given
// threshold.
func CleanupTerminatingPods(ctx context.Context, client kubernetes.Interface, namespace, component string, threshold time.Duration) {
	if client == nil || namespace == "" || component == "" {
		return
	}

	selector := labels.Set{"app.kubernetes.io/component": component}.AsSelector().String()
	ticker := time.NewTicker(threshold / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
			if err != nil {
				logrus.Errorf("(CleanupTerminatingPods) list pods error: %v", err)
				continue
			}

			for i := range pods.Items {
				pod := &pods.Items[i]
				if pod.DeletionTimestamp == nil {
					continue
				}
				if time.Since(pod.DeletionTimestamp.Time) < threshold {
					continue
				}

				logrus.Infof("(CleanupTerminatingPods) force deleting stuck pod %s/%s", pod.Namespace, pod.Name)

				if len(pod.Finalizers) > 0 {
					patch := []byte(`{"metadata":{"finalizers":null}}`)
					if _, err := client.CoreV1().Pods(pod.Namespace).Patch(ctx, pod.Name, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil && !apierrors.IsNotFound(err) {
						logrus.Errorf("(CleanupTerminatingPods) patch pod %s/%s error: %v", pod.Namespace, pod.Name, err)
					}
				}

				grace := int64(0)
				if err := client.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{GracePeriodSeconds: &grace}); err != nil && !apierrors.IsNotFound(err) {
					logrus.Errorf("(CleanupTerminatingPods) delete pod %s/%s error: %v", pod.Namespace, pod.Name, err)
				}
			}
		}
	}
}
