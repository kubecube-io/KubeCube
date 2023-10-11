/*
Copyright 2023 KubeCube Authors

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

package k8s

import (
	"github.com/gin-gonic/gin"

	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/warden/server/authproxy"
)

type Handler struct {
	*authproxy.Handler
}

func NewHandler() *Handler {
	h := new(Handler)
	cluster, err := multicluster.Interface().Get(constants.LocalCluster)
	if err != nil {
		clog.Fatal("get local cluster failed: %v", err)
	}
	authProxyHandler := &authproxy.Handler{}
	authProxyHandler.SetHandlerClient(cluster.Client)
	err = authProxyHandler.SetHandlerTS(cluster.Config)

	if err != nil {
		clog.Fatal("get local cluster auth proxy handler failed: %v", err)
	}
	h.Handler = authProxyHandler
	return h
}

func (h *Handler) LocalClusterProxy(c *gin.Context) {
	path := c.Param("path")
	c.Request.URL.Path = path
	h.ServeHTTP(c.Writer, c.Request)
}
