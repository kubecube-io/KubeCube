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
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcemanage "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/handle"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/enum"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"
	mgrclient "github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"github.com/kubecube-io/kubecube/pkg/utils/filter"
)

type Deployment struct {
	ctx             context.Context
	client          mgrclient.Client
	namespace       string
	filterCondition *filter.Condition
	lock            sync.Mutex
}

func init() {
	resourcemanage.SetExtendHandler(enum.DeploymentResourceType, handle)
}

func handle(extendCtx resourcemanage.ExtendContext) (interface{}, *errcode.ErrorInfo) {
	access := resources.NewSimpleAccess(extendCtx.Cluster, extendCtx.Username, extendCtx.Namespace)
	if allow := access.AccessAllow("apps", "deployments", "list"); !allow {
		return nil, errcode.ForbiddenErr
	}
	kubernetes := clients.Interface().Kubernetes(extendCtx.Cluster)
	if kubernetes == nil {
		return nil, errcode.ClusterNotFoundError(extendCtx.Cluster)
	}
	deployment := NewDeployment(kubernetes, extendCtx.Namespace, extendCtx.FilterCondition)
	return deployment.getExtendDeployments()
}

func NewDeployment(client mgrclient.Client, namespace string, condition *filter.Condition) Deployment {
	ctx := context.Background()
	return Deployment{
		ctx:             ctx,
		client:          client,
		namespace:       namespace,
		filterCondition: condition,
	}
}

// getExtendDeployments get extend deployments
func (d *Deployment) getExtendDeployments() (*unstructured.Unstructured, *errcode.ErrorInfo) {

	resultMap := make(map[string]interface{})
	// get deployment list from k8s cluster
	var deploymentList appsv1.DeploymentList
	err := d.client.Cache().List(d.ctx, &deploymentList, client.InNamespace(d.namespace))
	if err != nil {
		clog.Error("can not find info from cluster, %v", err)
		return nil, errcode.BadRequest(err)
	}
	// filterCondition list by selector/sort/page
	total, err := filter.GetEmptyFilter().FilterObjectList(&deploymentList, d.filterCondition)
	if err != nil {
		clog.Error("filterCondition deploymentList error, err: %s", err.Error())
		return nil, errcode.BadRequest(err)
	}
	// add pod status info
	resultList := d.addExtendInfo(deploymentList)
	resultMap["total"] = total
	resultMap["items"] = resultList

	return &unstructured.Unstructured{
		Object: resultMap,
	}, nil
}

func (d *Deployment) addExtendInfo(deploymentList appsv1.DeploymentList) []ExtentDeployment {
	resultList := make([]ExtentDeployment, 0)
	wg := &sync.WaitGroup{}
	for _, deployment := range deploymentList.Items {
		wg.Add(1)
		deployment := deployment
		go func() {
			result := d.getDeployExtendInfo(deployment)
			d.lock.Lock()
			resultList = append(resultList, result)
			d.lock.Unlock()
			wg.Done()
		}()
	}
	wg.Wait()
	return resultList
}

func (d *Deployment) getDeployExtendInfo(deployment appsv1.Deployment) ExtentDeployment {
	// get pod list by deployment
	realPodList, err := d.getPodByDeployment(deployment)
	if err != nil {
		clog.Info("add extend pods info to deployment %s fail, %v", deployment.Name, err)
		return ExtentDeployment{
			PodStatus{},
			deployment,
		}
	}

	// get warning event list by podList
	warningEventList, err := d.getWarningEventsByPodList(&realPodList)
	if err != nil {
		clog.Info("add extend warning events info to deployment %s fail, %v", deployment.Name, err)
	}

	// create podStatus map
	podsStatus := PodStatus{}
	podsStatus.Current = deployment.Status.Replicas
	podsStatus.Desired = deployment.Spec.Replicas
	var succeeded, running, pending, failed, unknown int32
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
	podsStatus.Succeeded = succeeded
	podsStatus.Running = running
	podsStatus.Pending = pending
	podsStatus.Failed = failed
	podsStatus.Unknown = unknown
	podsStatus.Warning = warningEventList

	// create result map
	result := ExtentDeployment{
		podsStatus,
		deployment,
	}
	return result
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
	// get pod by deployment matchLabel
	podList := corev1.PodList{}
	err := d.client.Cache().List(d.ctx, &podList, listOptions)
	if err != nil {
		return corev1.PodList{}, err
	}
	return podList, nil
}
