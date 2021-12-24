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
)

type EventInfo struct {
	EventName    string
	Description  string
	ResourceType string
}

func SetAuditInfo(c *gin.Context, eventInfo *EventInfo, resourceName string) *gin.Context {
	c.Set(EventName, eventInfo.EventName)
	c.Set(EventDescription, eventInfo.Description)
	c.Set(EventResourceType, eventInfo.ResourceType)
	c.Set(EventResourceName, resourceName)

	return c
}

var (
	CreateUser = &EventInfo{"createUser", "createUser", "user"}
	UpdateUser = &EventInfo{"updateUser", "updateUser", "user"}

	DeleteKey = &EventInfo{"deleteKey", "deleteKey", "key"}
)
