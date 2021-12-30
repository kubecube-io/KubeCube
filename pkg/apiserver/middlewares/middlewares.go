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

package middlewares

import (
	"github.com/gin-gonic/gin"
	"github.com/kubecube-io/kubecube/pkg/apiserver/middlewares/audit"
	"github.com/kubecube-io/kubecube/pkg/apiserver/middlewares/auth"
	"github.com/kubecube-io/kubecube/pkg/apiserver/middlewares/precheck"
	"github.com/kubecube-io/kubecube/pkg/apiserver/middlewares/recovery"
	"github.com/kubecube-io/kubecube/pkg/utils/env"
	"github.com/kubecube-io/kubecube/pkg/utils/international"
)

func SetUpMiddlewares(router *gin.Engine, managers *international.Gi18nManagers) {
	if router == nil {
		return
	}
	router.Use(precheck.PreCheck())
	router.Use(auth.Auth())
	if env.AuditIsEnable() {
		h := audit.NewHandler(managers)
		router.Use(h.Audit())
	}
	router.Use(recovery.Recovery())
}
