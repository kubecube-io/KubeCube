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

package scout

import (
	"github.com/gin-gonic/gin"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	"github.com/kubecube-io/kubecube/pkg/multicluster/scout"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"github.com/kubecube-io/kubecube/pkg/utils/response"
)

const subPath = "/scout"

func AddApisTo(root *gin.Engine) {
	r := root.Group(constants.ApiPathRoot + subPath)
	r.POST("/heartbeat", Scout)
}

// Scout collects information from wardens
// todo(weilaaa): to optimize it for reduce goroutine use
func Scout(c *gin.Context) {
	w := &scout.WardenInfo{}
	err := c.BindJSON(w)
	if err != nil {
		clog.Info("parse request body failed: %v", err)
		response.FailReturn(c, errcode.InvalidBodyFormat)
		return
	}

	internalCluster, err := multicluster.Interface().Get(w.Cluster)
	if err != nil && internalCluster == nil {
		clog.Debug("wait for cluster %v sync", w.Cluster)
		response.FailReturn(c, errcode.GetResourceError("cluster"))
		return
	}

	if internalCluster != nil {
		// use goroutine to fast return
		go func() {
			// send warden info to scout receiver
			internalCluster.Scout.Receiver <- *w
		}()
	}

	response.SuccessReturn(c, nil)
}
