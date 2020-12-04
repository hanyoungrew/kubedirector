// Copyright 2020 Hewlett Packard Enterprise Development LP

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package node

import (
	"context"

	"github.com/bluek8s/kubedirector/pkg/observer"
	"github.com/bluek8s/kubedirector/pkg/shared"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// syncNode runs the reconciliation logic. It is invoked because of a
// change in or addition of node instance, or a periodic polling to
// check on such a resource.
func (r *ReconcileNode) syncNode(
	reqLogger logr.Logger,
	node *corev1.Node,
) error {

	// Check availability of node.
	var ready corev1.ConditionStatus = corev1.ConditionUnknown
	for _, NodeCondition := range node.Status.Conditions {
		if NodeCondition.Type == corev1.NodeReady {
			ready = NodeCondition.Status
			break
		}
	}

	// If Node is now unavailable, start fencing procedure and relocation of pods
	if ready == corev1.ConditionUnknown {
		shared.LogInfof(
			reqLogger,
			node,
			shared.EventReasonNode,
			"node {%s} is unavailable, starting fencing/failover procedure",
			node.Name,
		)

		// assume that STONITH happens, either through fencing or by a STONITH daemon
		// e: = fenceNode(reqLogger, node)
		e := failoverNode(reqLogger, node)
		return e
	}
	return nil
}

// failoverNode runs the failOver logic to restart pods on new nodes.
func failoverNode(
	reqLogger logr.Logger,
	node *corev1.Node,
) error {
	shared.LogInfof(
		reqLogger,
		node,
		shared.EventReasonNode,
		"assuming node {%s} is fenced, starting failover procedure",
		node.Name,
	)

	// what goes in observer/executor?
	pods := &corev1.PodList{}
	shared.List(context.TODO(), pods)

	// For all pods running on node, patch all associated persistent volumes as RWX
	// then force delete the pod to let the StatefulSet controller reschedule it where available.

	// Alternatively, deleting the node object from Kubernetes will allow the
	// associated volume to be reclaimed by the newly rescheudled pod again.

	for _, pod := range pods.Items {
		if pod.Spec.NodeName == node.Name {
			for _, volume := range pod.Spec.Volumes {
				if volume.PersistentVolumeClaim != nil {
					pvcName := volume.PersistentVolumeClaim.ClaimName
					pvc, _ := observer.GetPVC(pod.Namespace, pvcName)
					pvName := pvc.Spec.VolumeName

					pv, _ := observer.GetPV(pvName)
					var rwm = make([]corev1.PersistentVolumeAccessMode, 0)
					rwm = append(rwm, corev1.ReadWriteMany)
					pv.Spec.AccessModes = rwm

					err := shared.Update(context.TODO(), pv)

					if err != nil {
						shared.LogError(
							reqLogger,
							err,
							pv,
							shared.EventReasonNoEvent,
							"failed to update persistent volume",
						)
						return err
					}
					shared.LogInfof(
						reqLogger,
						node,
						shared.EventReasonNode,
						"PersistentVolume {%s} was set to RWX",
						volume.Name,
					)
				}
			}

			// force delete pod to reschedule
			// add this to the executor module instead?
			toDelete := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      pod.Name,
					Namespace: pod.Namespace,
				},
			}
			var gracePeriod k8sClient.GracePeriodSeconds = 0
			shared.Delete(context.TODO(), toDelete, gracePeriod)
			shared.LogInfof(
				reqLogger,
				node,
				shared.EventReasonNode,
				"Pod {%s} was rescheduled",
				pod.Name,
			)
		}
	}
	return nil
}
