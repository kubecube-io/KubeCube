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

type ClusterState string

const (
	ClusterProcessing ClusterState = "processing"
	ClusterNormal     ClusterState = "normal"
	ClusterAbnormal   ClusterState = "abnormal"
)

// ClusterSpec defines the desired state of Cluster
type ClusterSpec struct {
	// KubeConfig contains cluster raw kubeConfig
	KubeConfig []byte `json:"kubeconfig,omitempty"`

	// Kubernetes API Server endpoint. Example: https://10.10.0.1:6443
	KubernetesAPIEndpoint string `json:"kubernetesAPIEndpoint,omitempty"`

	// cluster is member or not
	IsMemberCluster bool `json:"isMemberCluster,omitempty"`

	// describe cluster
	// +optional
	Description string `json:"description,omitempty"`

	// harbor address for cluster
	// +optional
	HarborAddr string `json:"harborAddr,omitempty"`

	// CNI the cluster used
	// +optional
	NetworkType string `json:"networkType,omitempty"`
}

// ClusterStatus defines the observed state of Cluster
type ClusterStatus struct {
	State         *ClusterState `json:"state,omitempty"`
	Reason        string        `json:"reason,omitempty"`
	LastHeartbeat *metav1.Time  `json:"lastHeartbeat,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:categories="cluster",scope="Cluster"
//+kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.state"

// Cluster is the Schema for the clusters API
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec,omitempty"`
	Status ClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterList contains a list of Cluster
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Cluster{}, &ClusterList{})
}
