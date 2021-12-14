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

package audit

import "github.com/gin-gonic/gin"

const (
	EventName         = "event"
	EventResourceType = "resourceType"
	EventResourceName = "resourceName"
	EventDescription  = "description"

	EventTypeUserWrite = "userwrite"
	EventAccountId     = "accountId"
)

type EventInfo struct {
	EventName    string
	Description  string
	ResourceType string
}

func SetAuditInfo(c *gin.Context, eventType *EventInfo, resourceName string) *gin.Context {
	c.Set(EventName, eventType.EventName)
	c.Set(EventDescription, eventType.Description)
	c.Set(EventResourceType, eventType.ResourceType)
	c.Set(EventResourceName, resourceName)

	return c
}

var (
	RegisterCluster = &EventInfo{"registerCluster", "注册集群", "cluster"}

	CreateKey = &EventInfo{"createKey", "创建密钥", "key"}
	DeleteKey = &EventInfo{"deleteKey", "删除密钥", "key"}

	Login           = &EventInfo{"login", "登录", "user"}
	CreateUser      = &EventInfo{"createUser", "创建用户", "user"}
	BatchCreateUser = &EventInfo{"batchCreateUser", "批量创建用户", "user"}
	UpdateUser      = &EventInfo{"updateUser", "更新用户", "user"}
)

var ResourceType = map[string]string{
	"deployments":               "无状态负载",
	"namespaces":                "namespace",
	"jobs":                      "任务",
	"cronjobs":                  "定时任务",
	"persistentvolumeclaims":    "存储声明",
	"customresourcedefinitions": "自定义资源",
	"horizontalpodautoscalers":  "自动伸缩(HPA)",
	"networkpolicies":           "网络策略",
	"statefulsets":              "有状态负载",
	"replicasets":               "副本集",
	"pods":                      "副本",
	"secret":                    "secret",
	"ingresses":                 "ingress",
	"services":                  "service",
	"configmaps":                "configmap",
}
