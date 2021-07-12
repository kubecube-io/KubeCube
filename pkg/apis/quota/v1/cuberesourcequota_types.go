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

package v1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GlobalNodesPool means
const GlobalNodesPool = "global"

type TargetObj struct {
	Name string     `json:"name,omitempty"`
	Kind TargetKind `json:"kind,omitempty"`
}

type TargetKind string

const (
	NodesPoolObj TargetKind = "NodesPool"
	TenantObj    TargetKind = "Tenant"
	ProjectObj   TargetKind = "Project"
)

// CubeResourceQuotaSpec defines the desired state of CubeResourceQuota
type CubeResourceQuotaSpec struct {
	// Hard is the set of desired hard limits for each named resource.
	// Its empty when TargetObj is NodesPoolObj
	Hard v1.ResourceList `json:"hard,omitempty"`

	// ParentQuota point to upper quota, its empty if current is top level
	// meanwhile PhysicalLimit will be used as limit condition
	// +optional
	ParentQuota string `json:"parentQuota,omitempty"`

	// Target point to the subject object quota to effect
	Target TargetObj `json:"target,omitempty"`
}

// CubeResourceQuotaStatus defines the observed state of CubeResourceQuota
type CubeResourceQuotaStatus struct {
	// Hard is the set of enforced hard limits for each named resource.
	// Limit always equals to request when TargetObj is NodesPoolObj
	// +optional
	Hard v1.ResourceList `json:"hard,omitempty"`
	// Used is the current observed total usage of the resource in the namespace
	// +optional
	Used v1.ResourceList `json:"used,omitempty"`

	// SubResourceQuotas contains child resource quotas of cube resource quota.
	// {name}.{namespace}.quota means resource quota
	// {name}.quota means cube resource quota
	// +optional
	SubResourceQuotas []string `json:"subResourceQuotas,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:categories="quota",scope="Cluster"

// CubeResourceQuota is the Schema for the cuberesourcequota API
type CubeResourceQuota struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CubeResourceQuotaSpec   `json:"spec,omitempty"`
	Status CubeResourceQuotaStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CubeResourceQuotaList contains a list of CubeResourceQuota
type CubeResourceQuotaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CubeResourceQuota `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CubeResourceQuota{}, &CubeResourceQuotaList{})
}
