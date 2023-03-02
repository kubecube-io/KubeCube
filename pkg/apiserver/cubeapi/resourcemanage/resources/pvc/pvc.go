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
	"encoding/json"
	"errors"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcemanage "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/handle"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/enum"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"github.com/kubecube-io/kubecube/pkg/utils/filter"
)

type PvcExtend struct {
	v1.PersistentVolumeClaim
	Pods  []v1.Pod `json:"pods,omitempty"`
	Total int      `json:"total,omitempty"`
}

func init() {
	resourcemanage.SetExtendHandler(enum.PvcResourceType, PvcHandle)
}

func PvcHandle(param resourcemanage.ExtendParams) (interface{}, error) {
	//access := resources.NewSimpleAccess(param.Cluster, param.Username, param.Namespace)
	//if allow := access.AccessAllow("", "persistentvolumeclaims", "list"); !allow {
	//	return nil, errors.New(errcode.ForbiddenErr.Message)
	//}
	kubernetes := clients.Interface().Kubernetes(param.Cluster)
	if kubernetes == nil {
		return nil, errors.New(errcode.ClusterNotFoundError(param.Cluster).Message)
	}
	pvc := NewPvc(kubernetes, param.Namespace, param.Filter)
	return pvc.GetPvc()
}

// GetPvc list pvcs, and add extend info that which pod mount this pvc
func (p *Pvc) GetPvc() (filter.K8sJson, error) {
	result := make(filter.K8sJson)
	pvcList := v1.PersistentVolumeClaimList{}
	err := p.client.Cache().List(p.ctx, &pvcList, &client.ListOptions{
		Namespace: p.namespace,
	})
	if err != nil {
		return nil, err
	}

	result["total"] = len(pvcList.Items)
	pvcListJson, err := json.Marshal(pvcList)
	if err != nil {
		return nil, err
	}

	filterResult := p.filter.FilterResult(pvcListJson)
	pvcList = v1.PersistentVolumeClaimList{}
	err = json.Unmarshal(filterResult, &pvcList)
	if err != nil {
		return nil, err
	}

	var pvcExtendList []PvcExtend
	for _, pvc := range pvcList.Items {
		workloadMap, err := p.GetPvcWorkloads(pvc.Name)
		if err != nil {
			return nil, err
		}
		// if response has pods, and result is pod array, then add it as extendInfo
		if podRes, ok := workloadMap["pods"]; ok {
			if pods, ok := podRes.([]v1.Pod); ok {
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
	result["items"] = pvcExtendList
	return result, nil
}
