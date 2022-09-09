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
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	jsoniter "github.com/json-iterator/go"
	corev1 "k8s.io/api/core/v1"
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
	ctx       context.Context
	client    mgrclient.Client
	namespace string
	filter    filter.Filter
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
	pod := NewPod(kubernetes, param.Namespace, param.Filter)
	if pod.filter.EnableFilter {
		err := pod.GetRs()
		if err != nil {
			return nil, errors.New(err.Error())
		}
	}
	result, err := pod.GetPods()
	return result, err
}

func NewPod(client mgrclient.Client, namespace string, filter filter.Filter) Pod {
	ctx := context.Background()
	return Pod{
		ctx:       ctx,
		client:    client,
		namespace: namespace,
		filter:    filter,
	}
}

func (d *Pod) GetRs() error {
	rsList := appsv1.ReplicaSetList{}
	err := d.client.Cache().List(d.ctx, &rsList, client.InNamespace(d.namespace))
	if err != nil {
		clog.Error("can not find rs from cluster, %v", err)
		return err
	}
	// filter list by selector/sort/page
	rsListJson, err := json.Marshal(rsList)
	if err != nil {
		clog.Error("convert replicaSetList to json fail, %v", err)
		return err
	}
	rsListMap := d.filter.FilterResultToMap(rsListJson, false, false)
	rsList = appsv1.ReplicaSetList{}
	reListMapJson, err := json.Marshal(rsListMap)
	if err != nil {
		clog.Error("convert replicaSetList to json fail, %v", err)
		return err
	}
	err = json.Unmarshal(reListMapJson, &rsList)
	if err != nil {
		clog.Error("convert replicaSetList from json fail, %v", err)
		return err
	}
	set := d.filter.Exact[ownerUidLabel]
	for _, rs := range rsList.Items {
		if set == nil {
			set = sets.NewString()
		}
		uid := rs.UID
		if len(uid) > 0 {
			set.Insert(string(uid))
		}
	}
	d.filter.Exact[ownerUidLabel] = set
	return nil
}

// get pods
func (d *Pod) GetPods() (filter.K8sJson, error) {

	//resultMap := make(resources.K8sJson)
	// get pod list from k8s cluster
	var podList corev1.PodList
	err := d.client.Cache().List(d.ctx, &podList, client.InNamespace(d.namespace))
	if err != nil {
		clog.Error("can not find info from cluster, %v", err)
		return nil, err
	}

	// filter list by selector/sort/page
	podListJson, err := json.Marshal(podList)
	if err != nil {
		clog.Error("convert deploymentList to json fail, %v", err)
		return nil, err
	}
	podListMap := d.filter.FilterResultToMap(podListJson, true, true)

	return podListMap, nil
}
