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

package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/kubecube-io/kubecube/pkg/authorizer/rbac"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	userinfo "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

var (
	i = rbac.NewDefaultResolver(constants.LocalCluster)
)

func Rbac() gin.HandlerFunc {
	return func(c *gin.Context) {
		clog.Debug("start check rest api permission")
		record := &authorizer.AttributesRecord{
			User: &userinfo.DefaultInfo{Name: ""},
			Verb: c.Request.Method,
			Path: c.Request.URL.Path,
		}
		i.Authorize(c.Request.Context(), record)
		c.Next()
	}
}
