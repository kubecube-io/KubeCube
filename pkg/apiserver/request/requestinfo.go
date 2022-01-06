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

package request

import (
	"github.com/gin-gonic/gin"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
)

const requestInfoKey = "requestInfoKey"

// ReqInfo go through the request lifetime inside context
type ReqInfo struct {
	cluster string
}

// K8ClientFrom retrieves client from context
func K8ClientFrom(c *gin.Context) client.Client {
	v, ok := c.Get(requestInfoKey)
	if !ok {
		clog.Warn("request info not found")
	}

	cluster := v.(*ReqInfo).cluster
	client := clients.Interface().Kubernetes(cluster)
	// err should be caught by recovery middleware

	return client
}

// WithK8Client sets client for context
func WithK8Client(c *gin.Context) *gin.Context {
	// todo: set cluster client according to request
	// master cluster client used by default
	r := &ReqInfo{cluster: constants.LocalCluster}

	c.Set(requestInfoKey, r)

	return c
}
