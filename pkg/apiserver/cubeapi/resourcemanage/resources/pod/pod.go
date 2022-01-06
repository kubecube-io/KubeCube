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

	jsoniter "github.com/json-iterator/go"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources"
	"github.com/kubecube-io/kubecube/pkg/clog"
	mgrclient "github.com/kubecube-io/kubecube/pkg/multicluster/client"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type Pod struct {
	ctx       context.Context
	client    mgrclient.Client
	namespace string
	filter    resources.Filter
}

func NewPod(client mgrclient.Client, namespace string, filter resources.Filter) Pod {
	ctx := context.Background()
	return Pod{
		ctx:       ctx,
		client:    client,
		namespace: namespace,
		filter:    filter,
	}
}

// get pods
func (d *Pod) GetPods() resources.K8sJson {

	//resultMap := make(resources.K8sJson)
	// get pod list from k8s cluster
	var podList corev1.PodList
	err := d.client.Cache().List(d.ctx, &podList, client.InNamespace(d.namespace))
	if err != nil {
		clog.Error("can not find info from cluster, %v", err)
		return nil
	}

	// filter list by selector/sort/page
	podListJson, err := json.Marshal(podList)
	if err != nil {
		clog.Error("convert deploymentList to json fail, %v", err)
		return nil
	}
	podListMap := d.filter.FilterResultToMap(podListJson)

	return podListMap
}
