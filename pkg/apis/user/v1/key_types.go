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

// KeySpec defines the desired state of Key
type KeySpec struct {
	SecretKey string `json:"secretKey,omitempty"`
	User      string `json:"user,omitempty"`
}

// KeyStatus defines the observed state of Key
type KeyStatus struct {
}

//+kubebuilder:object:root=true

//+kubebuilder:resource:categories="kubecube",scope="Cluster"
//+kubebuilder:printcolumn:name="User",type="string",JSONPath=".spec.user"
// Key is the Schema for the keys API
type Key struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KeySpec   `json:"spec,omitempty"`
	Status KeyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KeyList contains a list of Key
type KeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Key `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Key{}, &KeyList{})
}
