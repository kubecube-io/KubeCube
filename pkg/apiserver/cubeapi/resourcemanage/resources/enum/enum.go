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
)
