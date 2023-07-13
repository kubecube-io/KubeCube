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

package resourcemanage

import (
	"github.com/gin-gonic/gin"
	"github.com/kubecube-io/kubecube/pkg/apiserver/middlewares/audit"
	"github.com/kubecube-io/kubecube/pkg/utils/filter"
)

type ExtendContext struct {
	Cluster      string
	Namespace    string
	Username     string
	Action       string
	ResourceName string

	// todo: remove this customize field to suitable place
	NginxNamespace           string
	NginxTcpServiceConfigMap string
	NginxUdpServiceConfigMap string
	Body                     []byte
	NodeStatus               string

	GinContext      *gin.Context
	FilterCondition *filter.Condition
	AuditHandler    *audit.Handler
}
