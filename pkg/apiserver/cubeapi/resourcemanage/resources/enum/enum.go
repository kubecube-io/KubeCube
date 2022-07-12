/*
Copyright 2022 KubeCube Authors

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

package enum

type ResourceTypeEnum string

const (
	CronResourceType                  ResourceTypeEnum = "cronjobs"
	DeploymentResourceType            ResourceTypeEnum = "deployments"
	JobResourceType                   ResourceTypeEnum = "jobs"
	PodResourceType                   ResourceTypeEnum = "pods"
	PvcWorkLoadResourceType           ResourceTypeEnum = "pvcworkloads"
	ServiceResourceType               ResourceTypeEnum = "services"
	ExternalAccessResourceType        ResourceTypeEnum = "externalAccess"
	ExternalAccessAddressResourceType ResourceTypeEnum = "externalAccessAddress"
	PvcResourceType                   ResourceTypeEnum = "pvc"
	NodeResourceType                  ResourceTypeEnum = "nodes"
	NodeExportResourceType            ResourceTypeEnum = "nodeExport"
)
