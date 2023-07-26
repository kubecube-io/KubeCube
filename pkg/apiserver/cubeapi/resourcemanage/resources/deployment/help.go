/*
Copyright 2023 KubeCube Authors

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

package deployment

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ExtendEvent struct {
	Type string `json:"type,omitempty"`
	corev1.Event
}

type ExtentDeployment struct {
	PodStatus `json:"podStatus,omitempty"`
	appsv1.Deployment
}

type PodStatus struct {
	Current   int32         `json:"current,omitempty"`
	Desired   *int32        `json:"desired,omitempty"`
	Succeeded int32         `json:"succeeded,omitempty"`
	Running   int32         `json:"running,omitempty"`
	Pending   int32         `json:"pending,omitempty"`
	Failed    int32         `json:"failed,omitempty"`
	Unknown   int32         `json:"unknown,omitempty"`
	Warning   []ExtendEvent `json:"warning,omitempty"`
}

func isPodReadyOrSucceed(pod *corev1.Pod) bool {
	if pod.Status.Phase == "" {
		return true
	}

	if pod.Status.Phase == corev1.PodSucceeded {
		return true
	}

	if pod.Status.Phase == corev1.PodRunning {
		conditions := pod.Status.Conditions
		if len(conditions) == 0 {
			return true
		}
		for _, cond := range conditions {
			if cond.Type == "Ready" && cond.Status == "False" {
				return false
			}
		}
		return true
	}

	return false
}

func (d *Deployment) getWarningEventsByPodList(podList *corev1.PodList) ([]ExtendEvent, error) {
	// kubectl get ev --field-selector="involvedObject.uid=1a58441c-3c03-4267-85d1-a81f0c268d62,type=Warning"
	resultEventList := make([]ExtendEvent, 0)
	ctx, cancelFunc := context.WithTimeout(d.ctx, time.Second*10)
	defer cancelFunc()
	for _, pod := range podList.Items {
		if isPodReadyOrSucceed(&pod) {
			continue
		}
		fieldSelector := make(map[string]string)
		fieldSelector["involvedObject.uid"] = string(pod.GetUID())
		fieldSelector["type"] = "Warning"
		listOptions := &client.ListOptions{
			Namespace:     d.namespace,
			FieldSelector: fields.SelectorFromSet(fieldSelector),
		}

		eventList := corev1.EventList{}
		err := d.client.Direct().List(ctx, &eventList, listOptions)
		if err != nil {
			return nil, err
		}

		for _, event := range eventList.Items {
			tmp := ExtendEvent{
				"Warning",
				event,
			}
			resultEventList = append(resultEventList, tmp)
		}
	}
	return resultEventList, nil
}
