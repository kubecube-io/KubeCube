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

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/kubecube-io/kubecube/pkg/clog"
)

const (
	EventName         = "event"
	EventResourceType = "resourceType"
	EventResourceName = "resourceName"
	EventDescription  = "description"
	EventRequestBody  = "requestBody"
)

type EventInfo struct {
	EventName    string
	Description  string
	ResourceType string
}

func SetAuditInfo(c *gin.Context, eventInfo *EventInfo, resourceName string, RequestBody interface{}) *gin.Context {
	c.Set(EventName, eventInfo.EventName)
	c.Set(EventDescription, eventInfo.Description)
	c.Set(EventResourceType, eventInfo.ResourceType)
	c.Set(EventResourceName, resourceName)

	if RequestBody != nil {
		body, err := json.Marshal(RequestBody)
		if err != nil {
			clog.Warn("json marshal failed for %v: %v", eventInfo.EventName, err)
		} else {
			c.Set(EventRequestBody, string(body))
		}
	}

	return c
}

var (
	CreateUser       = &EventInfo{"createUser", "createUser", "user"}
	UpdateUser       = &EventInfo{"updateUser", "updateUser", "user"}
	DeleteKey        = &EventInfo{"deleteKey", "deleteKey", "key"}
	CreateKey        = &EventInfo{"createKey", "createKey", "key"}
	CreateConfigMap  = &EventInfo{"createConfigMap", "createConfigMap", "configmap"}
	DeleteConfigMap  = &EventInfo{"deleteConfigMap", "deleteConfigMap", "configmap"}
	UpdateConfigMap  = &EventInfo{"updateConfigMap", "updateConfigMap", "configmap"}
	RolloutConfigMap = &EventInfo{"rolloutConfigMap", "rolloutConfigMap", "configmap"}
)
