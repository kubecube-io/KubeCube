/*
Copyright 2022 KubeCube Authors

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

package node

import (
	"context"
	"errors"

	jsoniter "github.com/json-iterator/go"
	corev1 "k8s.io/api/core/v1"

	resourcemanage "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/handle"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/enum"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"
	mgrclient "github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"github.com/kubecube-io/kubecube/pkg/utils/filter"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type Node struct {
	ctx    context.Context
	client mgrclient.Client
	filter filter.Filter
}

func init() {
	resourcemanage.SetExtendHandler(enum.NodeResourceType, handle)
}

func handle(param resourcemanage.ExtendParams) (interface{}, error) {
	//access := resources.NewSimpleAccess(param.Cluster, param.Username, param.Namespace)
	//if allow := access.AccessAllow("", "nodes", "list"); !allow {
	//	return nil, errors.New(errcode.ForbiddenErr.Message)
	//}
	kubernetes := clients.Interface().Kubernetes(param.Cluster)
	if kubernetes == nil {
		return nil, errors.New(errcode.ClusterNotFoundError(param.Cluster).Message)
	}
	node := NewNode(kubernetes, param.Filter)
	return node.GetExtendNodes()
}

func NewNode(client mgrclient.Client, filter filter.Filter) Node {
	ctx := context.Background()
	return Node{
		ctx:    ctx,
		client: client,
		filter: filter,
	}
}

func (node *Node) GetExtendNodes() (filter.K8sJson, error) {
	resultMap := make(filter.K8sJson)

	// get deployment list from k8s cluster
	nodeList := corev1.NodeList{}
	err := node.client.Cache().List(node.ctx, &nodeList)
	if err != nil {
		clog.Error("can not find node in cluster, %v", err)
		return nil, err
	}
	resultMap["total"] = len(nodeList.Items)
	resultMap["items"] = addExtendInfo(&nodeList)
	// filter list by selector/sort/page
	resultMapJson, err := json.Marshal(resultMap)
	if err != nil {
		clog.Error("convert nodeExtendList to json fail,nodeExtendList: %+v, err: %+v", resultMap, err)
		return nil, err
	}
	filterResult := node.filter.FilterResult(resultMapJson)
	resultMap = make(filter.K8sJson)
	err = json.Unmarshal(filterResult, &resultMap)
	if err != nil {
		clog.Error("convert json to result map fail, %v", err)
		return nil, err
	}
	return resultMap, nil
}

func addExtendInfo(nodeList *corev1.NodeList) filter.K8sJsonArr {
	items := make(filter.K8sJsonArr, 0)
	for _, node := range nodeList.Items {
		// parse job status
		status := ParseNodeStatus(node)

		// add extend info
		extendInfo := make(filter.K8sJson)
		extendInfo["status"] = status

		// add node info and extend info
		result := make(filter.K8sJson)
		result["metadata"] = node.ObjectMeta
		result["spec"] = node.Spec
		result["status"] = node.Status
		result["extendInfo"] = extendInfo

		//add to list
		items = append(items, result)
	}
	return items
}

func ParseNodeStatus(node corev1.Node) (status string) {
	if node.Spec.Unschedulable {
		return UnscheduledStatus
	}
	if node.Status.Conditions == nil {
		return ""
	}
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
			return Normal
		}
	}
	return AbNormal
}
