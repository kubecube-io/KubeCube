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

type ComponentConfig struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Status    string `json:"status,omitempty"`
	PkgName   string `json:"pkgName,omitempty"`
	Env       string `json:"env,omitempty"`
}

type DeployResult struct {
	Name    string `json:"name,omitempty"`
	Status  string `json:"status,omitempty"`
	Result  string `json:"result,omitempty"`
	Message string `json:"message,omitempty"`
}

// HotplugSpec defines the desired state of Hotplug
type HotplugSpec struct {
	Component []ComponentConfig `json:"component,omitempty"`
}

// HotplugStatus defines the observed state of Hotplug
type HotplugStatus struct {
	Phase   string          `json:"phase,omitempty"`
	Results []*DeployResult `json:"results,omitempty"`
}

//+kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Hotplug is the Schema for the hotplugs API
// +kubebuilder:resource:categories="kubecube",scope="Cluster"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Hotplug struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HotplugSpec   `json:"spec,omitempty"`
	Status HotplugStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HotplugList contains a list of Hotplug
type HotplugList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Hotplug `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Hotplug{}, &HotplugList{})
}
