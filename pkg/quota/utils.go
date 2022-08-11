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

package quota

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const SubFix = "quota"

const Finalizer = "finalizers.kubecube.io/quota"

const ResourceNvidiaGPU v1.ResourceName = "requests.nvidia.com/gpu"

var ResourceNames = []v1.ResourceName{
	// request and limit
	v1.ResourceRequestsCPU,
	v1.ResourceLimitsCPU,
	v1.ResourceRequestsMemory,
	v1.ResourceLimitsMemory,
	v1.ResourceRequestsEphemeralStorage,
	v1.ResourceLimitsEphemeralStorage,
	v1.ResourceRequestsStorage,
	// general
	v1.ResourceCPU,
	v1.ResourceMemory,
	v1.ResourceStorage,
	v1.ResourceEphemeralStorage,
	// gpu
	ResourceNvidiaGPU,

	// counts
	v1.ResourcePods,
	// todo: support resource quota bellow in the future
	//v1.ResourceConfigMaps,
	//v1.ResourceSecrets,
	//v1.ResourceReplicationControllers,
	//v1.ResourceQuotas,
	//v1.ResourcePersistentVolumeClaims,
	//v1.ResourceServices,
	//v1.ResourceServicesNodePorts,
	//v1.ResourceServicesLoadBalancers,
}

// ZeroQ give the value of zero
func ZeroQ() resource.Quantity {
	return resource.MustParse("0")
}

// ClearQuotas clear all quotas of give
func ClearQuotas(l v1.ResourceList) v1.ResourceList {
	for k := range l {
		l[k] = ZeroQ()
	}

	return l
}
