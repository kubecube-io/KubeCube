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

type UserState string
type LoginType string
type Language string

const (
	NormalState    UserState = "normal"
	ForbiddenState UserState = "forbidden"

	NormalLogin LoginType = "normal"
	OpenIdLogin LoginType = "openId"
	LDAPLogin   LoginType = "ldap"
	Oauth2Login LoginType = "oauth2"

	English Language = "en"
	Chinese Language = "zh"
)

// UserSpec defines the desired state of User
type UserSpec struct {
	Password    string `json:"password,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Email       string `json:"email,omitempty"`
	Phone       string `json:"phone,omitempty"`
	// The preferred written or spoken language for the user: chinese/english
	Language Language `json:"language,omitempty"`
	// Login method used, normal/openId/ldap
	LoginType LoginType `json:"loginType,omitempty"`
	State     UserState `json:"state,omitempty"`
}

// UserStatus defines the observed state of User
type UserStatus struct {
	// The user status, normal/forbidden
	LastLoginTime *metav1.Time `json:"lastLoginTime,omitempty"`
	LastLoginIP   string       `json:"lastLoginIP,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:categories="user",scope="Cluster"
//+kubebuilder:printcolumn:name="LoginType",type="string",JSONPath=".spec.loginType"
//+kubebuilder:printcolumn:name="LastLoginTime",type="date",JSONPath=".status.lastLoginTime"

// User is the Schema for the users API
type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserSpec   `json:"spec,omitempty"`
	Status UserStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// UserList contains a list of User
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}

func init() {
	SchemeBuilder.Register(&User{}, &UserList{})
}
