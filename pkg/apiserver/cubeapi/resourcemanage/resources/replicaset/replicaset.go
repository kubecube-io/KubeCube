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

package replicaset

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

type Replicaset struct {
	ctx             context.Context
	client          mgrclient.Client
	namespace       string
	filterCondition *filter.Condition
}

func init() {
	resourcemanage.SetExtendHandler(enum.ReplicasetType, handle)
}

func handle(param resourcemanage.ExtendContext) (interface{}, *errcode.ErrorInfo) {
	access := resources.NewSimpleAccess(param.Cluster, param.Username, param.Namespace)
	if allow := access.AccessAllow("apps", "replicasets", "list"); !allow {
		return nil, errcode.ForbiddenErr
	}
	kubernetes := clients.Interface().Kubernetes(param.Cluster)
	if kubernetes == nil {
		return nil, errcode.ClusterNotFoundError(param.Cluster)
	}
	replicaset := NewReplicaset(kubernetes, param.Namespace, param.FilterCondition)
	return replicaset.getExtendJobs()
}

func NewReplicaset(client mgrclient.Client, namespace string, condition *filter.Condition) *Replicaset {
	ctx := context.Background()
	return &Replicaset{
		ctx:             ctx,
		client:          client,
		namespace:       namespace,
		filterCondition: condition,
	}
}

// getExtendJobs get extend deployments
func (r *Replicaset) getExtendJobs() (*unstructured.Unstructured, *errcode.ErrorInfo) {
	resultMap := make(map[string]interface{})

	// get deployment list from k8s cluster
	list := appsv1.ReplicaSetList{}
	err := r.client.Cache().List(r.ctx, &list, client.InNamespace(r.namespace))
	if err != nil {
		clog.Error("can not find replicaset in %s from cluster, %v", r.namespace, err)
		return nil, errcode.BadRequest(err)
	}

	// filterCondition list by selector/sort/page
	total, err := filter.GetEmptyFilter().FilterObjectList(&list, r.filterCondition)
	if err != nil {
		clog.Error("filterCondition replicaSetList error, err: %s", err.Error())
		return nil, errcode.BadRequest(err)
	}

	resultMap["total"] = total
	resultMap["items"] = list.Items

	return &unstructured.Unstructured{
		Object: resultMap,
	}, nil
}
