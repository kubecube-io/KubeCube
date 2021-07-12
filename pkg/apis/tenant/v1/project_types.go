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

// ProjectSpec defines the desired state of Project
type ProjectSpec struct {
	// +kubebuilder:validation:MaxLength=100
	// +kubebuilder:validation:MinLength=1
	DisplayName string `json:"displayName,omitempty"`

	// +kubebuilder:validation:MaxLength=200
	// +kubebuilder:validation:MinLength=1
	Description string `json:"description,omitempty"`

	Namespace string `json:"namespace,omitempty"`
}

// ProjectStatus defines the observed state of Project
type ProjectStatus struct {
}

//+kubebuilder:object:root=true

// Project is the Schema for the projects API
// +kubebuilder:resource:categories="kubecube",scope="Cluster"
// +kubebuilder:printcolumn:name="Tenant",type=string,JSONPath=`.metadata.labels.kubecube\.io/tenant`
// +kubebuilder:printcolumn:name="DisplayName",type=string,JSONPath=`.spec.displayName`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="Namespace",type="string",JSONPath=".spec.namespace"
type Project struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProjectSpec   `json:"spec,omitempty"`
	Status ProjectStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ProjectList contains a list of Project
type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Project `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Project{}, &ProjectList{})
}
