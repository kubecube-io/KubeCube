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
package job

import (
	"context"

	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources"
	mgrclient "github.com/kubecube-io/kubecube/pkg/multicluster/client"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Pvc struct {
	ctx       context.Context
	client    mgrclient.Client
	namespace string
	filter    resources.Filter
}

func NewPvc(client mgrclient.Client, namespace string, filter resources.Filter) Pvc {
	ctx := context.Background()
	return Pvc{
		ctx:       ctx,
		client:    client,
		namespace: namespace,
		filter:    filter,
	}
}

// get extend deployments
func (p *Pvc) GetPvcWorkloads(pvcName string) resources.K8sJson {
	result := make(resources.K8sJson)
	var pods []corev1.Pod
	var podList corev1.PodList
	err := p.client.Cache().List(p.ctx, &podList, client.InNamespace(p.namespace))
	if err != nil {
		return nil
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
	return result
}
