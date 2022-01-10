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
package deployment

import (
	"context"

	jsoniter "github.com/json-iterator/go"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources"
	"github.com/kubecube-io/kubecube/pkg/clog"
	mgrclient "github.com/kubecube-io/kubecube/pkg/multicluster/client"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type Deployment struct {
	ctx       context.Context
	client    mgrclient.Client
	namespace string
	filter    resources.Filter
}

func NewDeployment(client mgrclient.Client, namespace string, filter resources.Filter) Deployment {
	ctx := context.Background()
	return Deployment{
		ctx:       ctx,
		client:    client,
		namespace: namespace,
		filter:    filter,
	}
}

// get extend deployments
func (d *Deployment) GetExtendDeployments() resources.K8sJson {

	resultMap := make(resources.K8sJson)
	// get deployment list from k8s cluster
	var deploymentList appsv1.DeploymentList
	err := d.client.Cache().List(d.ctx, &deploymentList, client.InNamespace(d.namespace))
	if err != nil {
		clog.Error("can not find info from cluster, %v", err)
		return nil
	}
	resultMap["total"] = len(deploymentList.Items)

	// filter list by selector/sort/page
	deploymentListJson, err := json.Marshal(deploymentList)
	if err != nil {
		clog.Error("convert deploymentList to json fail, %v", err)
		return nil
	}
	deploymentListJson = d.filter.FilterResult(deploymentListJson)
	err = json.Unmarshal(deploymentListJson, &deploymentList)
	if err != nil {
		clog.Error("convert json to deploymentList fail, %v", err)
		return nil
	}

	// add pod status info
	resultList := d.addExtendInfo(deploymentList)

	resultMap["items"] = resultList

	return resultMap
}

func (d *Deployment) addExtendInfo(deploymentList appsv1.DeploymentList) resources.K8sJsonArr {
	resultList := make(resources.K8sJsonArr, 0)
	for _, deployment := range deploymentList.Items {
		// get pod list by deployment
		realPodList, err := d.getPodByDeployment(deployment)
		if err != nil {
			clog.Info("add extend pods info to deployment %s fail, %v", deployment.Name, err)
			continue
		}

		// get warning event list by podList
		warningEventList, err := d.getWarningEventsByPodList(realPodList)
		if err != nil {
			clog.Info("add extend warning events info to deployment %s fail, %v", deployment.Name, err)
			warningEventList = make(resources.K8sJsonArr, 0)
		}

		// create podStatus map
		podsStatus := make(resources.K8sJson)
		podsStatus["current"] = deployment.Status.Replicas
		podsStatus["desired"] = deployment.Spec.Replicas
		var succeeded, running, pending, failed, unknown int
		for _, p := range realPodList.Items {
			switch p.Status.Phase {
			case corev1.PodSucceeded:
				succeeded++
			case corev1.PodRunning:
				running++
			case corev1.PodPending:
				pending++
			case corev1.PodFailed:
				failed++
			default:
				unknown++
			}
		}
		podsStatus["succeeded"] = succeeded
		podsStatus["running"] = running
		podsStatus["pending"] = pending
		podsStatus["failed"] = failed
		podsStatus["unknown"] = unknown

		podsStatus["warning"] = warningEventList

		// create result map
		result := make(resources.K8sJson)
		result["metadata"] = deployment.ObjectMeta
		result["spec"] = deployment.Spec
		result["status"] = deployment.Status
		result["podStatus"] = podsStatus
		resultList = append(resultList, result)
	}
	return resultList
}

// get podList by deployment, return real pod list
func (d *Deployment) getPodByDeployment(deployment appsv1.Deployment) (corev1.PodList, error) {
	if deployment.Spec.Selector == nil || deployment.Spec.Selector.MatchLabels == nil {
		return corev1.PodList{}, nil
	}

	// get deployment matchLabel
	listOptions := &client.ListOptions{
		Namespace:     d.namespace,
		LabelSelector: labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels),
	}

	// get replicas by deployment matchLabel
	rsList := appsv1.ReplicaSetList{}
	err := d.client.Cache().List(d.ctx, &rsList, listOptions)
	if err != nil {
		return corev1.PodList{}, err
	}
	var rsUidList []types.UID
	for _, rs := range rsList.Items {
		ownerReferences := rs.ObjectMeta.OwnerReferences
		if ownerReferences == nil {
			continue
		}
		for _, ownerReference := range ownerReferences {
			if ownerReference.UID == deployment.UID {
				rsUidList = append(rsUidList, rs.UID)
			}
		}
	}

	// get pod by deployment matchLabel
	podList := corev1.PodList{}
	err = d.client.Cache().List(d.ctx, &podList, listOptions)
	if err != nil {
		return corev1.PodList{}, err
	}
	// get pod by replicas
	var realPodList corev1.PodList
	for _, pod := range podList.Items {
		ownerReferences := pod.ObjectMeta.OwnerReferences
		if ownerReferences == nil {
			continue
		}
		for _, ownerReference := range ownerReferences {
			for _, rsUid := range rsUidList {
				if ownerReference.UID == rsUid {
					realPodList.Items = append(realPodList.Items, pod)
				}
			}
		}
	}
	return realPodList, nil
}

func (d *Deployment) getWarningEventsByPodList(podList corev1.PodList) (resources.K8sJsonArr, error) {
	// kubectl get ev --field-selector="involvedObject.uid=1a58441c-3c03-4267-85d1-a81f0c268d62,type=Warning"
	resultEventList := make(resources.K8sJsonArr, 0)
	for _, pod := range podList.Items {
		if isPodReadyOrSucceed(pod) {
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
		err := d.client.Direct().List(d.ctx, &eventList, listOptions)
		if err != nil {
			return nil, err
		}

		for _, event := range eventList.Items {
			eventMap := make(resources.K8sJson)
			eventMap["type"] = "Warning"
			eventMap["reason"] = event.Reason
			eventMap["message"] = event.Message
			eventMap["object"] = event.InvolvedObject.Name
			eventMap["creationTimestamp"] = event.CreationTimestamp
			resultEventList = append(resultEventList, eventMap)
		}
	}
	return resultEventList, nil
}

func isPodReadyOrSucceed(pod corev1.Pod) bool {
	if pod.Status.Phase == "" {
		return true
	}

	if pod.Status.Phase == "Succeeded" {
		return true
	}

	if pod.Status.Phase == "Running" {
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
