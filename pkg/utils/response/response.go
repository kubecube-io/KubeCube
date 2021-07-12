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

package response

import (
	"github.com/gin-gonic/gin"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"net/http"
)

type SuccessInfo struct {
	Message string `json:"message"`
}

func SuccessReturn(c *gin.Context, obj interface{}) {
	if obj == nil {
		obj = SuccessInfo{
			Message: "success",
		}
	}
	c.JSON(http.StatusOK, obj)
	c.Abort()
}

func FailReturn(c *gin.Context, errorInfo *errcode.ErrorInfo, params ...interface{}) {
	c.JSON(errorInfo.Code, errcode.New(errorInfo, params...))
	c.Abort()
}
