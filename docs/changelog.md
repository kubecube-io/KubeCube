# v1.2.0

## Feature
- move resource filter to util [#99](https://github.com/kubecube-io/KubeCube/pull/99)
- adapt hnc ga and use labels spread feature [#98](https://github.com/kubecube-io/KubeCube/pull/98)
- k8s version adaptive convert [#97](https://github.com/kubecube-io/KubeCube/pull/97)
- cluster info add cpu and mem used quota [#96](https://github.com/kubecube-io/KubeCube/pull/96)
- add the access to restapi [#92](https://github.com/kubecube-io/KubeCube/pull/92)
- add audit to yamldeploy, create key, service extend external access[#89](https://github.com/kubecube-io/KubeCube/pull/89)
- kubecube client interface add restful, restmappper, discovery clients [#87](https://github.com/kubecube-io/KubeCube/pull/87)
- yaml deploy change to restclient and use username to auth [#85](https://github.com/kubecube-io/KubeCube/pull/85)
- support set ingress domain suffix [#84](https://github.com/kubecube-io/KubeCube/pull/84)
- Support warden register [#83](https://github.com/kubecube-io/KubeCube/pull/83)
- enhance multi cluster manager pkg [#82](https://github.com/kubecube-io/KubeCube/pull/82)
- update local up script [#81](https://github.com/kubecube-io/KubeCube/pull/81)
- make audit report international [#80](https://github.com/kubecube-io/KubeCube/pull/80)

## BugFix
- Fix version conversion [#101](https://github.com/kubecube-io/KubeCube/pull/101)
- request to login makes error logs in audit middleware [#100](https://github.com/kubecube-io/KubeCube/pull/100)
- Rename clinet.go to client.go [#95](https://github.com/kubecube-io/KubeCube/pull/95)
- remove not exist subResourceQuota [#94](https://github.com/kubecube-io/KubeCube/pull/94)
- update jwt version to dodge attack [#91](https://github.com/kubecube-io/KubeCube/pull/91)
- fix audit middleware to a goroutine [#90](https://github.com/kubecube-io/KubeCube/pull/90)

## Dependencies

- hnc v1.0
- nginx-ingress v0.46.0
- helm 3.x
- metrics-server v0.4.1
- elasticsearch 7.8
- kubecube-monitoring 15.4.8
- thanos 3.18.0
- logseer v1.0.0
- logagent v1.0.0
- kubecube-audit 1.2.0

# v1.1.0

## Feature
- auth-proxy in front of k8s-apiserver for proxying kubectl and restful access ([#73](https://github.com/kubecube-io/KubeCube/pull/73), [#67](https://github.com/kubecube-io/KubeCube/pull/67))
- change algorithm of quota [#72](https://github.com/kubecube-io/KubeCube/pull/72)
- add operation of e2e init and end [#68](https://github.com/kubecube-io/KubeCube/pull/68)
- clean up: implement jwt utils to interface ([#64](https://github.com/kubecube-io/KubeCube/pull/64), [#65](https://github.com/kubecube-io/KubeCube/pull/65))
- github login of oAuth2 support [#60](https://github.com/kubecube-io/KubeCube/pull/60)
- warden-init-container can use charts pkg offline or download it from remote [#57](https://github.com/kubecube-io/KubeCube/pull/57)

## BugFix
- fix that can not add customize ClusterRole [#71](https://github.com/kubecube-io/KubeCube/pull/71)
- hide user password when login [#66](https://github.com/kubecube-io/KubeCube/pull/66)
- close response body after do audit middlewares [#55](https://github.com/kubecube-io/KubeCube/pull/55/files)
- fix hotplug result status error && fix proxy http and https in kubecube [#52](https://github.com/kubecube-io/KubeCube/pull/52)

## Dependencies

- hnc v0.8.0-kubecube.1.1
- nginx-ingress v0.46.0
- helm 3.x
- metrics-server v0.4.1
- elasticsearch 7.8
- kubecube-monitoring 15.4.8
- thanos 3.18.0
- logseer v1.0.0
- logagent v1.0.0
- kubecube-audit 1.1.0

# v1.0.3
## BugFix
- fix tenant namespace annotation

## Dependencies

- hnc v0.8.0-kubecube.1.1
- nginx-ingress v0.46.0
- helm 3.x
- metrics-server v0.4.1
- elasticsearch 7.8
- kubecube-monitoring 15.4.8
- thanos 3.18.0
- logseer v1.0.0
- logagent v1.0.0
- kubecube-audit 1.0.0

# V1.0.2

## BugFix

- `KubeCube:` fix the problem of resource quota webhook since conformance test

## Dependencies

- hnc v0.8.0-kubecube.1
- nginx-ingress v0.46.0
- helm 3.x
- metrics-server v0.4.1
- elasticsearch 7.8
- kubecube-monitoring 15.4.8
- thanos 3.18.0
- logseer v1.0.0
- logagent v1.0.0
- kubecube-audit 1.0.0

# V1.0.1

2021-8-19

## BugFix

- `KubeCube:`fix the bug that use old script to add memeber cluster
- `KubeCube:`fix delete cluster failed

## Dependencies

- hnc v0.8.0-kubecube.1
- nginx-ingress v0.46.0
- helm 3.x
- metrics-server v0.4.1
- elasticsearch 7.8
- kubecube-monitoring 15.4.8
- thanos 3.18.0
- logseer v1.0.0
- logagent v1.0.0
- kubecube-audit 1.0.0

# V1.0.0

2021-8-6

## Feature

- `Observability:`add control plane component monitoring
- `Observability:`add administrator alert configuration

## BugFix

- `KubeCube:`decouple ClusterInfo api from metric server
- `Warden:`fix hotplug {{.cluster}} injected error in the member cluster
- `Warden:`added configmap to record components status for fron

## Optimization

- `Warden:`optimize performance of warden informer
- `Warden:`optimize status in the hotplug manifest
- `KubeCube:`optimize performance of cluster controller

## Dependencies

- hnc v0.8.0-kubecube.1
- nginx-ingress v0.46.0
- helm 3.x
- metrics-server v0.4.1
- elasticsearch 7.8
- kubecube-monitoring 15.4.8
- thanos 3.18.0
- logseer v1.0.0
- logagent v1.0.0
- kubecube-audit 1.0.0

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


- hnc v0.8.0-kubecube.1
- nginx-ingress v0.46.0
- helm 3.x
- metrics-server v0.4.1
- elasticsearch 7.8
- kubecube-monitoring 15.4.8
- thanos 3.18.0
- logseer v1.0.0
- logagent v1.0.0
- kubecube-audit 1.0.0-rc0
