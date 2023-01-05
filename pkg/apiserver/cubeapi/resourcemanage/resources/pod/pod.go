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
	"errors"
	jsoniter "github.com/json-iterator/go"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
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

var json = jsoniter.ConfigCompatibleWithStandardLibrary

const ownerUidLabel = "metadata.ownerReferences.uid"

type Pod struct {
	ctx             context.Context
	client          mgrclient.Client
	namespace       string
	filterCondition *filter.Condition
}

func init() {
	resourcemanage.SetExtendHandler(enum.PodResourceType, Handle)
}

func Handle(param resourcemanage.ExtendParams) (interface{}, error) {
	access := resources.NewSimpleAccess(param.Cluster, param.Username, param.Namespace)
	if allow := access.AccessAllow("", "pods", "list"); !allow {
		return nil, errors.New(errcode.ForbiddenErr.Message)
	}
	kubernetes := clients.Interface().Kubernetes(param.Cluster)
	if kubernetes == nil {
		return nil, errors.New(errcode.ClusterNotFoundError(param.Cluster).Message)
	}
	pod := NewPod(kubernetes, param.Namespace, param.FilterCondition)
	if pod.filterCondition.Exact[ownerUidLabel].Len() > 0 {
		err := pod.GetRs()
		if err != nil {
			return nil, errors.New(err.Error())
		}
	}
	result, err := pod.GetPods()
	return result, err
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

func (d *Pod) GetRs() error {
	if len(d.filterCondition.Exact) == 0 && len(d.filterCondition.Exact) == 0 {
		return nil
	}
	rsList := appsv1.ReplicaSetList{}
	err := d.client.Cache().List(d.ctx, &rsList, client.InNamespace(d.namespace))
	if err != nil {
		clog.Error("can not find rs from cluster, %v", err)
		return err
	}
	// filterCondition list by selector/sort/page

	_, err = filter.GetEmptyFilter().FilterObjectList(&rsList, d.filterCondition)
	if err != nil {
		clog.Error("filterCondition rsList error, err: %s", err.Error())
		return err
	}
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

// get pods
func (d *Pod) GetPods() (*unstructured.Unstructured, error) {

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
	resultMap["items"] = podList.Items
	return &unstructured.Unstructured{Object: resultMap}, nil
}
