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
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"

	"github.com/kubecube-io/kubecube/pkg/authorizer/rbac"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"github.com/kubecube-io/kubecube/pkg/utils/response"
)

type handler struct {
	rbac.Interface
	client.Client
}

func NewHandler() *handler {
	h := new(handler)
	h.Interface = rbac.NewDefaultResolver(constants.LocalCluster)
	h.Client = clients.Interface().Kubernetes(constants.LocalCluster)
	return h
}

func Rbac() gin.HandlerFunc {
	h := NewHandler()
	return func(c *gin.Context) {
		if WithinWhiteList(c.Request.URL, c.Request.Method, AuthWhiteList) {
			c.Next()
			return
		}
		clog.Info("user %v start check rest api permission, path: %v", c.GetString(constants.EventAccountId), c.Request.URL.Path)
		record := &authorizer.AttributesRecord{
			User: &user.DefaultInfo{Name: c.GetString(constants.EventAccountId)},
			Verb: c.Request.Method,
			Path: c.Request.URL.Path,
		}
		d, _, err := h.Authorize(c.Request.Context(), record)
		if err != nil {
			clog.Error(err.Error())
			response.FailReturn(c, errcode.InternalServerError)
			return
		}
		if d != authorizer.DecisionAllow {
			clog.Debug("user %v has no permission for the path: %v", c.GetString(constants.EventAccountId), c.Request.URL.Path)
			response.FailReturn(c, errcode.ForbiddenErr)
			return
		}
		clog.Info("user %v check permission success, path %v", c.GetString(constants.EventAccountId), c.Request.URL.Path)
		c.Next()
	}
}
