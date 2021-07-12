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
	KubeCube = "kubecube"

	// root route
	ApiPathRoot = "/api/v1/cube"

	// kubecube default namespace
	CubeNamespace = "kubecube-system"

	// pivot cluster name
	PivotCluster = "pivot-cluster"

	// warden deployment name
	WardenDeployment = "warden"

	// default pivot cube host
	DefaultPivotCubeHost = "cube.kubecube.io"

	// default pivot cube headless svc
	DefaultPivotCubeClusterIPSvc = "kubecube.kubecube-system:7443"
	DefaultAuditSvc              = "audit.kubecube-system:8888"

	HttpHeaderContentType        = "Content-type"
	HttpHeaderContentDisposition = "Content-Disposition"
	HttpHeaderContentTypeOctet   = "application/octet-stream"

	ClusterLabel = "kubecube.io/cluster"

	// TenantLabel represent which tenant resource relate with
	TenantLabel = "kubecube.io/tenant"

	// ProjectLabel represent which project resource relate with
	ProjectLabel = "kubecube.io/project"

	// CubeQuotaLabel point to CubeResourceQuota
	CubeQuotaLabel = "kubecube.io/quota"

	// RbacLabel indicates the resource of rbac is related with kubecube
	RbacLabel = "kubecube.io/rbac"

	RoleLabel = "kubecube.io/role"

	CrdLabel = "kubecube.io/crds"

	// SyncLabel
	SyncLabel = "kubecube.io/sync"

	K8sResourceVersion   = "v1"
	K8sResourceNamespace = "namespaces"

	// audit
	EventName          = "event"
	EventTypeUserWrite = "userwrite"
	EventResourceType  = "resourceType"
	EventAccountId     = "accountId"

	// user
	AuthorizationHeader        = "Authorization"
	DefaultTokenExpireDuration = 3600 // 1 hour

	// build-in cluster role
	PlatformAdmin = "platform-admin"
	TenantAdmin   = "tenant-admin"
	ProjectAdmin  = "project-admin"
	Reviewer      = "reviewer"

	TenantAdminCluster  = "tenant-admin-cluster"
	ProjectAdminCluster = "project-admin-cluster"
	ReviewerCluster     = "reviewer-cluster"
)
