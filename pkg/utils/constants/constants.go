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

package constants

const (
	// KubeCube all the begin
	KubeCube = "kubecube"

	// Warden is willing to kubecube
	Warden = "warden"

	// ApiPathRoot the root api route
	ApiPathRoot = "/api/v1/cube"

	// CubeNamespace kubecube default namespace
	CubeNamespace = "kubecube-system"

	// PivotCluster pivot cluster name
	// Deprecated
	PivotCluster = "pivot-cluster"

	// LocalCluster the internal cluster where program stand with
	LocalCluster = "_local_cluster"

	// DefaultPivotCubeClusterIPSvc default pivot cube svc
	DefaultPivotCubeClusterIPSvc = "kubecube.kubecube-system:7443"

	DefaultAuditURL = "http://audit.kubecube-system:8888/api/v1/cube/audit/cube"
)

// http content
const (
	HttpHeaderContentType        = "Content-type"
	HttpHeaderContentDisposition = "Content-Disposition"
	HttpHeaderContentTypeOctet   = "application/octet-stream"

	ImpersonateUserKey  = "Impersonate-User"
	ImpersonateGroupKey = "Impersonate-Group"
)

// audit and user constant
const (
	EventName          = "event"
	EventTypeUserWrite = "userwrite"
	EventResourceType  = "resourceType"
	EventAccountId     = "accountId"
	EventObjectName    = "objectName"
	EventRespBody      = "responseBody"

	AuthorizationHeader        = "Authorization"
	DefaultTokenExpireDuration = 3600 // 1 hour
)

// k8s api resources
const (
	K8sResourceVersion   = "v1"
	K8sResourceNamespace = "namespaces"
	K8sResourcePod       = "pods"

	K8sKindClusterRole    = "ClusterRole"
	K8sKindRole           = "Role"
	K8sKindServiceAccount = "ServiceAccount"

	K8sGroupRBAC = "rbac.authorization.k8s.io"
)

// rbac related constant
const (
	PlatformAdmin = "platform-admin"
	TenantAdmin   = "tenant-admin"
	ProjectAdmin  = "project-admin"
	Reviewer      = "reviewer"

	TenantAdminCluster  = "tenant-admin-cluster"
	ProjectAdminCluster = "project-admin-cluster"
	ReviewerCluster     = "reviewer-cluster"

	PlatformAdminAgLabel = "rbac.authorization.k8s.io/aggregate-to-platform-admin"
	TenantAdminAgLabel   = "rbac.authorization.k8s.io/aggregate-to-tenant-admin"
	ProjectAdminAgLabel  = "rbac.authorization.k8s.io/aggregate-to-project-admin"
	ReviewerAgLabel      = "rbac.authorization.k8s.io/aggregate-to-reviewer"
)

const (
	// ClusterLabel indicates the resource which cluster relate with
	ClusterLabel = "kubecube.io/cluster"

	// TenantLabel represent which tenant resource relate with
	TenantLabel = "kubecube.io/tenant"

	// ProjectLabel represent which project resource relate with
	ProjectLabel = "kubecube.io/project"

	// CubeQuotaLabel point to CubeResourceQuota
	CubeQuotaLabel = "kubecube.io/quota"

	// RbacLabel indicates the resource of rbac is related with kubecube
	RbacLabel = "kubecube.io/rbac"
	// RoleLabel indicates the role of rbac policy
	RoleLabel = "kubecube.io/role"

	// CrdLabel indicates the crds kubecube need to dispatch
	CrdLabel = "kubecube.io/crds"

	// SyncAnnotation use for sync logic of warden
	SyncAnnotation = "kubecube.io/sync"
)

const (
	// CubeNodeTaint is node taint that managed by KubeCube
	CubeNodeTaint = "node.kubecube.io"
)

// hnc related conest
const (
	// HncInherited means resource is inherited form upon namespace by hnc
	HncInherited = "hnc.x-k8s.io/inherited-from"
)

// rbac role verbs
const (
	// AllVerb all verbs
	AllVerb = "*"
	// CreateVerb create resource
	CreateVerb = "create"
	// DeleteVerb delete resource
	DeleteVerb = "delete"
	// ListVerb list resource
	ListVerb = "list"
)
