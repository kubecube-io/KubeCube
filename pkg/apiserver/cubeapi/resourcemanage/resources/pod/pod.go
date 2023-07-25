/*
Copyright 2021 KubeCube Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pod

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/node"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcemanage "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/handle"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/enum"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"
	mgrclient "github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"github.com/kubecube-io/kubecube/pkg/utils/filter"
)

const ownerUidLabel = "metadata.ownerReferences.uid"

type ExtendPod struct {
	Reason string `json:"reason,omitempty"`
	corev1.Pod
}
type Pod struct {
	ctx             context.Context
	client          mgrclient.Client
	namespace       string
	filterCondition *filter.Condition
}

func init() {
	resourcemanage.SetExtendHandler(enum.PodResourceType, handle)
}

func handle(param resourcemanage.ExtendContext) (interface{}, *errcode.ErrorInfo) {
	access := resources.NewSimpleAccess(param.Cluster, param.Username, param.Namespace)
	if allow := access.AccessAllow("", "pods", "list"); !allow {
		return nil, errcode.ForbiddenErr
	}
	kubernetes := clients.Interface().Kubernetes(param.Cluster)
	if kubernetes == nil {
		return nil, errcode.ClusterNotFoundError(param.Cluster)
	}
	pod := NewPod(kubernetes, param.Namespace, param.FilterCondition)
	if pod.filterCondition.Exact[ownerUidLabel].Len() > 0 {
		err := pod.getRs()
		if err != nil {
			return nil, errcode.BadRequest(err)
		}
	}
	result, err := pod.getPods()
	if err != nil {
		return nil, errcode.BadRequest(err)
	}

	return result, nil
}

func NewPod(client mgrclient.Client, namespace string, filter *filter.Condition) Pod {
	ctx := context.Background()
	return Pod{
		ctx:             ctx,
		client:          client,
		namespace:       namespace,
		filterCondition: filter,
	}
}

// getRs If you want to query the pods associated with an owner reference UID,
// you need to first query the corresponding replicaset
// and then use the replicaset's UID to search for the corresponding pod UID, achieving a cascading query.
func (d *Pod) getRs() error {
	val, ok := d.filterCondition.Exact[ownerUidLabel]
	if !ok {
		return nil
	}
	// Only the owner reference UID is needed as a filtering condition.
	condition := &filter.Condition{
		Exact: map[string]sets.String{ownerUidLabel: val},
	}
	rsList := appsv1.ReplicaSetList{}
	err := d.client.Cache().List(d.ctx, &rsList, client.InNamespace(d.namespace))
	if err != nil {
		clog.Error("can not find rs from cluster, %v", err)
		return err
	}
	_, err = filter.GetEmptyFilter().FilterObjectList(&rsList, condition)
	if err != nil {
		clog.Error("filterCondition rsList error, err: %s", err.Error())
		return err
	}
	// Insert the uid of the replicaset as a filtering condition into the filtering condition set.
	set := d.filterCondition.Exact[ownerUidLabel]
	for _, rs := range rsList.Items {
		if set == nil {
			set = sets.NewString()
		}
		uid := rs.UID
		if len(uid) > 0 {
			set.Insert(string(uid))
		}
	}
	d.filterCondition.Exact[ownerUidLabel] = set
	return nil
}

func (d *Pod) getPods() (*unstructured.Unstructured, error) {

	// get pod list from k8s cluster
	resultMap := make(map[string]interface{})
	var podList corev1.PodList
	err := d.client.Cache().List(d.ctx, &podList, client.InNamespace(d.namespace))
	if err != nil {
		clog.Error("can not find info from cluster, %v", err)
		return nil, err
	}

	// filterCondition list by selector/sort/page
	total, err := filter.GetEmptyFilter().FilterObjectList(&podList, d.filterCondition)
	if err != nil {
		clog.Error("filterCondition podList error, err: %s", err.Error())
		return nil, err
	}

	// add pod status info
	resultMap["total"] = total
	items := make([]ExtendPod, 0)
	for _, pod := range podList.Items {
		items = append(items, ExtendPod{
			Reason: getPodReason(pod),
			Pod:    pod,
		})
	}
	resultMap["items"] = items
	return &unstructured.Unstructured{Object: resultMap}, nil
}

// getPodReason return The aggregate status of the containers in this pod.
func getPodReason(pod corev1.Pod) string {
	restarts := 0
	readyContainers := 0
	lastRestartDate := metav1.NewTime(time.Time{})

	reason := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		reason = pod.Status.Reason
	}

	// If the Pod carries {type:PodScheduled, reason:WaitingForGates}, set reason to 'SchedulingGated'.
	// SchedulingGated not in k8s 1.19, use a hard code replace.
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodScheduled && condition.Reason == "SchedulingGated" {
			reason = "SchedulingGated"
		}
	}

	initializing := false
	for i := range pod.Status.InitContainerStatuses {
		container := pod.Status.InitContainerStatuses[i]
		restarts += int(container.RestartCount)
		if container.LastTerminationState.Terminated != nil {
			terminatedDate := container.LastTerminationState.Terminated.FinishedAt
			if lastRestartDate.Before(&terminatedDate) {
				lastRestartDate = terminatedDate
			}
		}
		switch {
		case container.State.Terminated != nil && container.State.Terminated.ExitCode == 0:
			continue
		case container.State.Terminated != nil:
			// initialization is failed
			if len(container.State.Terminated.Reason) == 0 {
				if container.State.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Init:Signal:%d", container.State.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("Init:ExitCode:%d", container.State.Terminated.ExitCode)
				}
			} else {
				reason = "Init:" + container.State.Terminated.Reason
			}
			initializing = true
		case container.State.Waiting != nil && len(container.State.Waiting.Reason) > 0 && container.State.Waiting.Reason != "PodInitializing":
			reason = "Init:" + container.State.Waiting.Reason
			initializing = true
		default:
			reason = fmt.Sprintf("Init:%d/%d", i, len(pod.Spec.InitContainers))
			initializing = true
		}
		break
	}
	if !initializing {
		restarts = 0
		hasRunning := false
		for i := len(pod.Status.ContainerStatuses) - 1; i >= 0; i-- {
			container := pod.Status.ContainerStatuses[i]

			restarts += int(container.RestartCount)
			if container.LastTerminationState.Terminated != nil {
				terminatedDate := container.LastTerminationState.Terminated.FinishedAt
				if lastRestartDate.Before(&terminatedDate) {
					lastRestartDate = terminatedDate
				}
			}
			if container.State.Waiting != nil && container.State.Waiting.Reason != "" {
				reason = container.State.Waiting.Reason
			} else if container.State.Terminated != nil && container.State.Terminated.Reason != "" {
				reason = container.State.Terminated.Reason
			} else if container.State.Terminated != nil && container.State.Terminated.Reason == "" {
				if container.State.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Signal:%d", container.State.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("ExitCode:%d", container.State.Terminated.ExitCode)
				}
			} else if container.Ready && container.State.Running != nil {
				hasRunning = true
				readyContainers++
			}
		}

		// change pod status back to "Running" if there is at least one container still reporting as "Running" status
		if reason == constants.CompletedPodStatus && hasRunning {
			if hasPodReadyCondition(pod.Status.Conditions) {
				reason = string(corev1.PodRunning)
			} else {
				reason = constants.NotReadyPodStatus
			}
		}
	}

	if pod.DeletionTimestamp != nil && pod.Status.Reason == node.NodeUnreachablePodReason {
		reason = constants.UnknownPodStatus
	} else if pod.DeletionTimestamp != nil {
		reason = constants.TerminatingPodStatus
	}

	return reason
}

func hasPodReadyCondition(conditions []corev1.PodCondition) bool {
	for _, condition := range conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
