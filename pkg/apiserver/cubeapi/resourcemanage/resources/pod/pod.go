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
	podListMap := d.filter.FilterResultToMap(podListJson)

	return podListMap, nil
}
