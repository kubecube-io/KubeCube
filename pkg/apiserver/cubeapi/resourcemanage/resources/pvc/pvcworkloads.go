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

package pvc

import (
	"context"
	"errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcemanage "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/handle"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/enum"
	"github.com/kubecube-io/kubecube/pkg/clients"
	mgrclient "github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"github.com/kubecube-io/kubecube/pkg/utils/filter"
)

type Pvc struct {
	ctx       context.Context
	client    mgrclient.Client
	namespace string
	filter    *filter.Filter
}

func init() {
	resourcemanage.SetExtendHandler(enum.PvcWorkLoadResourceType, Handle)
}

func Handle(param resourcemanage.ExtendParams) (interface{}, error) {
	access := resources.NewSimpleAccess(param.Cluster, param.Username, param.Namespace)
	if allow := access.AccessAllow("", "persistentvolumeclaims", "list"); !allow {
		return nil, errors.New(errcode.ForbiddenErr.Message)
	}
	kubernetes := clients.Interface().Kubernetes(param.Cluster)
	if kubernetes == nil {
		return nil, errors.New(errcode.ClusterNotFoundError(param.Cluster).Message)
	}
	pvc := NewPvc(kubernetes, param.Namespace, param.Filter)
	return pvc.GetPvcWorkloads(param.ResourceName)
}

func NewPvc(client mgrclient.Client, namespace string, filter *filter.Filter) Pvc {
	ctx := context.Background()
	return Pvc{
		ctx:       ctx,
		client:    client,
		namespace: namespace,
		filter:    filter,
	}
}

// GetPvcWorkloads get extend deployments
func (p *Pvc) GetPvcWorkloads(pvcName string) (*unstructured.Unstructured, error) {
	result := make(map[string]interface{})
	var pods []corev1.Pod
	var podList corev1.PodList
	err := p.client.Cache().List(p.ctx, &podList, client.InNamespace(p.namespace))
	if err != nil {
		return nil, err
	}
	for _, pod := range podList.Items {
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim == nil {
				continue
			}
			claimName := volume.PersistentVolumeClaim.ClaimName
			if claimName == pvcName {
				pods = append(pods, pod)
				break
			}
		}
	}
	result["pods"] = pods
	result["total"] = len(pods)
	return &unstructured.Unstructured{Object: result}, nil
}
