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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ExternalResourceSpec defines the desired state of ExternalResource
type ExternalResourceSpec struct {
	// Namespaced the scope of resource
	Namespaced bool `json:"namespaced,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:categories="extension",scope="Cluster"

// ExternalResource for mapping non-k8s resource so that
// we can use it as general k8s resource to rbac
type ExternalResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ExternalResourceSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// ExternalResourceList contains a list of ExternalResource
type ExternalResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExternalResource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ExternalResource{}, &ExternalResourceList{})
}
