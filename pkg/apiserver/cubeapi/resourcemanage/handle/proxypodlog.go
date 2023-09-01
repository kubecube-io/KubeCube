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

package resourcemanage

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"github.com/kubecube-io/kubecube/pkg/utils/filter"
	"github.com/kubecube-io/kubecube/pkg/utils/response"
	v1 "k8s.io/api/core/v1"
)

// NOTE: This file is copied from k8s.io/kubernetes/dashboard/src/app/backend/resource/container/logs.go.
// We have expanded some function and delete some we did not use, such as HandleLogs.

type ProxyPodLog struct {
	ctx             context.Context
	client          client.Client
	namespace       string
	filterCondition *filter.Condition
}

func NewProxyPodLog(client client.Client, namespace string, condition *filter.Condition) ProxyPodLog {
	ctx := context.Background()
	return ProxyPodLog{
		ctx:             ctx,
		client:          client,
		namespace:       namespace,
		filterCondition: condition,
	}
}

func (podLog *ProxyPodLog) HandleLogs(c *gin.Context) {
	k8sClient := podLog.client.ClientSet()

	namespace := c.Param("namespace")
	pod := c.Param("resourceName")
	container := c.Query("container")
	tailLines := c.Query("tailLines")
	timestamps := c.Query("timestamps")
	limitBytes := c.Query("limitBytes")
	sinceSeconds := c.Query("sinceSeconds")
	follow := c.Query("follow")

	if len(tailLines) == 0 {
		response.FailReturn(c, errcode.ParamsMissing("tailLines"))
		return
	}
	lines, err := strconv.ParseInt(tailLines, 10, 64)
	if err != nil {
		response.FailReturn(c, errcode.ParamsInvalid(err))
		return
	}
	isTimestamps, _ := strconv.ParseBool(timestamps)
	isFollow, _ := strconv.ParseBool(follow)
	if len(limitBytes) == 0 {
		response.FailReturn(c, errcode.ParamsMissing("limitBytes"))
		return
	}
	limit, err := strconv.ParseInt(limitBytes, 10, 64)
	if err != nil {
		response.FailReturn(c, errcode.ParamsInvalid(err))
		return
	}
	if len(sinceSeconds) == 0 {
		response.FailReturn(c, errcode.ParamsMissing("sinceSeconds"))
		return
	}
	seconds, err := strconv.ParseInt(sinceSeconds, 10, 64)
	if err != nil {
		response.FailReturn(c, errcode.ParamsInvalid(err))
		return
	}
	logOptions := &v1.PodLogOptions{
		Container:    container,
		Follow:       isFollow,
		Timestamps:   isTimestamps,
		TailLines:    &lines,
		LimitBytes:   &limit,
		SinceSeconds: &seconds,
	}
	logStream, err := k8sClient.CoreV1().Pods(namespace).GetLogs(pod, logOptions).Stream(c)
	if err != nil {
		clog.Warn("get log details fail: %v", err)
		response.FailReturn(c, errcode.BadRequest(err))
		return
	}
	defer func(logStream io.ReadCloser) {
		_ = logStream.Close()
	}(logStream)
	writer := c.Writer
	header := writer.Header()
	header.Set(constants.HttpHeaderTransferEncoding, constants.HttpHeaderChunked)
	header.Set(constants.HttpHeaderContentType, constants.HttpHeaderTextHtml)
	r := bufio.NewReader(logStream)
	for {
		bytes, err := r.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			clog.Warn("read log fail: %v", err)
			response.FailReturn(c, errcode.BadRequest(err))
			return
		}
		_, err = writer.Write(bytes)
		if err != nil {
			clog.Warn("write log fail: %v", err)
			response.FailReturn(c, errcode.BadRequest(err))
			return
		}
		writer.Flush()
	}
}
