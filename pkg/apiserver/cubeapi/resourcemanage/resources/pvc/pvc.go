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

package pvc

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcemanage "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/handle"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/enum"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/pod"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"github.com/kubecube-io/kubecube/pkg/utils/filter"
)

type PvcExtend struct {
	v1.PersistentVolumeClaim
	Pods  []pod.ExtendPod `json:"pods,omitempty"`
	Total int             `json:"total,omitempty"`
}

func init() {
	resourcemanage.SetExtendHandler(enum.PvcResourceType, pvcHandle)
}

func pvcHandle(param resourcemanage.ExtendContext) (interface{}, *errcode.ErrorInfo) {
	access := resources.NewSimpleAccess(param.Cluster, param.Username, param.Namespace)
	if allow := access.AccessAllow("", "persistentvolumeclaims", "list"); !allow {
		return nil, errcode.ForbiddenErr
	}
	kubernetes := clients.Interface().Kubernetes(param.Cluster)
	if kubernetes == nil {
		return nil, errcode.ClusterNotFoundError(param.Cluster)
	}
	pvc := NewPvc(kubernetes, param.Namespace, param.FilterCondition)
	return pvc.getPvc()
}

// getPvc list pvcs, and add extend info that which pod mount this pvc
func (p *Pvc) getPvc() (*unstructured.Unstructured, *errcode.ErrorInfo) {
	result := make(map[string]interface{})
	pvcList := v1.PersistentVolumeClaimList{}
	err := p.client.Cache().List(p.ctx, &pvcList, &client.ListOptions{
		Namespace: p.namespace,
	})
	if err != nil {
		return nil, errcode.BadRequest(err)
	}

	total, err := filter.GetEmptyFilter().FilterObjectList(&pvcList, p.filterCondition)
	if err != nil {
		clog.Error("filterCondition pvcList error, err: %s", err.Error())
		return nil, errcode.BadRequest(err)
	}

	var pvcExtendList []PvcExtend
	for _, pvc := range pvcList.Items {
		workloadMap, errInfo := p.getPvcWorkloads(pvc.Name)
		if errInfo != nil {
			return nil, errInfo
		}
		// if response has pods, and result is pod array, then add it as extendInfo
		if podRes, ok := workloadMap.Object["pods"]; ok {
			if pods, ok := podRes.([]pod.ExtendPod); ok {
				extend := PvcExtend{
					PersistentVolumeClaim: pvc,
					Pods:                  pods,
					Total:                 len(pods),
				}
				pvcExtendList = append(pvcExtendList, extend)
			} else {
				clog.Error("get pvc mounted pod err, res: %+v", workloadMap)
			}
		}
	}
	result["total"] = total
	result["items"] = pvcExtendList
	return &unstructured.Unstructured{Object: result}, nil
}
