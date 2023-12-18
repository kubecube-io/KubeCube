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
	GitHubLogin LoginType = "github"

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
	Wechat    string    `json:"wechat,omitempty"`

	// ScopeBindings indicates user relationships with tenant,project or platform
	// +optional
	ScopeBindings []ScopeBinding `json:"scopeBindings,omitempty"`
}

type BindingScopeType string

const (
	TenantScope   BindingScopeType = "tenant"
	ProjectScope  BindingScopeType = "project"
	PlatformScope BindingScopeType = "platform"
)

type ScopeBinding struct {
	// ScopeType the binding scope type that support tenant,project and platform.
	ScopeType BindingScopeType `json:"scopeType"`

	// ScopeName the specific scope name.
	ScopeName string `json:"scopeName"`

	// Role the rbac role name.
	Role string `json:"role"`
}

// UserStatus defines the observed state of User
type UserStatus struct {
	// The user status, normal/forbidden
	LastLoginTime *metav1.Time `json:"lastLoginTime,omitempty"`
	LastLoginIP   string       `json:"lastLoginIP,omitempty"`

	// BelongTenants indicates the user belongs to those tenants.
	// +optional
	BelongTenants []string `json:"belongTenants,omitempty"`

	// BelongProjects indicates the user belongs to those projects.
	// +optional
	// Deprecated: use BelongProjectInfos instead
	BelongProjects []string `json:"belongProjects,omitempty"`

	// BelongProjectInfos indicates the user belongs to those projects.
	// +optional
	BelongProjectInfos []ProjectInfo `json:"belongProjectInfos,omitempty"`

	// PlatformAdmin indicates the user is platform admin or not.
	// +optional
	PlatformAdmin bool `json:"platformAdmin,omitempty"`
}

type ProjectInfo struct {
	Project string `json:"project,omitempty"`
	Tenant  string `json:"tenant,omitempty"`
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

func (u *User) IsUserPlatformScope() bool {
	platformScope := false
	for _, scope := range u.Spec.ScopeBindings {
		if scope.ScopeType == PlatformScope {
			platformScope = true
			break
		}
	}
	return platformScope
}
