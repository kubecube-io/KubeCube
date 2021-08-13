# KubeCube
[![License](http://img.shields.io/badge/license-apache%20v2-blue.svg)](https://https://github.com/kubecube-io/kubecube/blob/main/LICENSE)

![logo](./logo.png)

> [English](../README.md) | 中文文档

KubeCube 是一个开源的企业级容器平台，为企业提供 Kubernetes 资源可视化管理以及统一的多集群多租户管理功能。KubeCube 可以简化应用部署、管理应用的生命周期和提供丰富的监控界面和日志审计功能，帮助企业快速构建一个强大和功能丰富的容器云管理平台。

![dashboard](./dashboard.png)

## 核心能力

- **开箱即用**
  - 学习曲线平缓，集成统一认证鉴权、多集群管理、监控、日志、告警等功能，释放生产力
  - 运维友好，提供 Kubernetes 资源可视化管理和统一运维，具备全面的自监控能力
  - 快速部署，提供一键式 [All in One](https://www.kubecube.io/docs/quick-start/installation/) 最小化部署模式，提供生产级[高可用部署](https://www.kubecube.io/docs/installation-guide/install-on-multi-node/)
- **[多租户管理](https://www.kubecube.io/docs/user-guide/administration/tenant/)**
  - 提供租户、项目、空间多级模型，以满足企业内资源隔离和软件项目管理需求
  - 基于多租户模型，提供权限控制、资源共享/隔离等能力
- **统一的[多 Kubernetes 集群管理](https://www.kubecube.io/docs/user-guide/administration/k8s-cluster/multi-k8s-cluster-mgr/)**
  - 提供多 Kubernetes 集群的中央管理面板，支持集群导入
  - 在多 Kubernetes 集群中提供统一的身份认证和拓展 Kubernetes 原生 RBAC 能力实现[访问控制](https://www.kubecube.io/docs/user-guide/administration/role/)
  - 通过 WebConsole、CloudShell 快速管理集群资源
- **集群自治**
  - 当 KubeCube 管理集群停机维护时，各业务集群可保持自治，保持正常的访问控制，业务 Pod 无感知
- **功能[热插拔](https://www.kubecube.io/docs/installation-guide/enable-plugins/)**
  - 提供最小化安装，用户可以根据需求随时开关功能
  - 可热插拔，无需重启服务
- **多种接入方式**
  - 支持 [Open API](https://www.kubecube.io/docs/developer-guide/openapi-guide/)：方便对接用户现有系统
  - 兼容 Kubernetes 原生 API：无缝兼容现有 Kubernetes 工具链，如 kubectl
- **无供应商锁定**
  - 可导入任意标准 Kubernetes 集群，更好的支持多云/混合云
- **其他功能**
  - [操作审计](https://www.kubecube.io/docs/user-guide/administration/audit/)
  - 丰富的可观测性功能

## 解决什么问题

- **帮助企业上云**

  简化学习曲线，帮助企业以较小的成本完成容器云平台搭建，实现应用快速上云需求，辅助企业推动应用上云。

- **资源隔离、配额和权限管理**

  多租户管理提供租户、项目和空间三个层级的资源隔离、配额管理和权限控制，完全适配企业级私有云建设的资源和权限管控需求。

- **集群规模无限制**

  统一的容器云管理平台，可以管理多个业务 Kubernetes 集群，数量不设上限。既能通过横向扩容新增 Kubernetes 集群的方式解决单个 Kubernetes 集群规模的限制，又可以满足不同业务条线要求独占集群的需求。

- **丰富的可观测性**

  支持集群维度和应用维度的监控告警和日志采集，提供丰富的工作负载监控指标界面和集群维度的监控界面，提供灵活的日志查询能力。

## 产品架构

KubeCube 产品由 KubeCube Service、Warden、CloudShell 和 AuditLog Server 等组件组成，除了 Warden 部署在各个 Kubernetes 集群充当认证代理，其余组件均部署在管理集群。

下图描述的 KubeCube 整体产品架构，包括与用户的交互，与 Kubernetes API Server 交互，Prometheus 监控和自研日志采集组件。

![architecture](./architecture.png)

## 快速入门

1、[部署环境要求](https://www.kubecube.io/docs/installation-guide/requirement/)

2、[All In One 部署](https://www.kubecube.io/docs/quick-start/installation/)

3、[快速体验](https://www.kubecube.io/docs/quick-start/quick-experience/)

## 参与开发

[贡献](https://www.kubecube.io/docs/developer-guide/contributing/)

## 讨论与反馈

欢迎加入微信群交流。

<img src="./kubecube-wechat.png" alt="kubecube-wechat" style="max-width:20%;" />


## 开源协议

```
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
```