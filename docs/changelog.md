

# V1.0.0-rc0

2021-7-16

## Feature

### Platform management

- Add account management
- Add tenant, project management, and support level-by-level transfer of permissions
- Support OpenAPI
- Add record the operation audit log, supporting KubeCube  and K8s API Server
- Support the component hotplug

### K8s cluster management

- Add permission management, based on K8s RBAC capability expansion
- Add K8s cluster management and resource synchronization between clusters
- Add tenant and namespace quota management

### K8s resource management

- Add workload management
- Add service and discovery management
- Add configuration management
- Add PVC management
- Add Yaml orchestration function

### Observable

- Add the prometheus monitoring function
- Add fail alarm function
- Add log collect and query function

### Online operation and maintenance tools

- Add terminal capabilities
- Provide K8s fault diagnosis tool

### Other non-functional

- Provide All-in-One lightweight deployment mode and provide high-availability deployment mode in production environment 
- Provide usage documentation, link [kubecube.io](https://www.kubecube.io/)
- With single test and e2e test
- Conduct laboratory stability and performance tests

## Dependencies

- hnc v0.8.0-kubecube.0.1
- nginx-ingress v0.46.0
- helm 3.x
- metrics-server v0.4.1
- elasticsearch 7.8
- kubecube-monitoring 15.4.8
- thanos 3.18.0
- logseer v1.0.0
- logagent v1.0.0
- kubecube-audit 1.0.0-rc0

