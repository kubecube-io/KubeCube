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

package precheck

import (
	"github.com/gin-gonic/gin"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"github.com/kubecube-io/kubecube/pkg/utils/response"
)

// PreCheck do cluster health check, early return
// if cluster if unhealthy
func PreCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		cluster := fetchCluster(c)

		if len(cluster) > 0 {
			_, err := multicluster.Interface().Get(cluster)
			if err != nil {
				clog.Warn(err.Error())
				response.FailReturn(c, errcode.InternalServerError)
				return
			}
		}
	}
}

func fetchCluster(c *gin.Context) string {
	const key = "cluster"

	switch {
	case len(c.Query(key)) > 0:
		return c.Query(key)
	case len(c.Param(key)) > 0:
		return c.Param(key)
	case len(c.PostForm(key)) > 0:
		return c.PostForm(key)
	default:
		// todo(weilaaa): to support body
		return ""
	}
}
