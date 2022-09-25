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
	"context"
	"io"
	"io/ioutil"
	"strconv"

	"github.com/gin-gonic/gin"
	v1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/scheme"

	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"github.com/kubecube-io/kubecube/pkg/utils/filter"
	"github.com/kubecube-io/kubecube/pkg/utils/log"
	"github.com/kubecube-io/kubecube/pkg/utils/response"
)

// NOTE: This file is copied from k8s.io/kubernetes/dashboard/src/app/backend/resource/container/logs.go.
// We have expanded some function and delete some we did not use, such as HandleLogs.

type PodLog struct {
	ctx       context.Context
	client    client.Client
	namespace string
	filter    *filter.Filter
}

func NewPodLog(client client.Client, namespace string, filter *filter.Filter) PodLog {
	ctx := context.Background()
	return PodLog{
		ctx:       ctx,
		client:    client,
		namespace: namespace,
		filter:    filter,
	}
}

func (podLog *PodLog) HandleLogs(c *gin.Context) {
	k8sClient := podLog.client.ClientSet()

	namespace := c.Param("namespace")
	podID := c.Param("resourceName")
	containerID := c.Query("containerName")

	refTimestamp := c.Query("referenceTimestamp")
	if refTimestamp == "" {
		refTimestamp = log.NewestTimestamp
	}

	refLineNum, err := strconv.Atoi(c.Query("referenceLineNum"))
	if err != nil {
		refLineNum = 0
	}
	usePreviousLogs := c.Query("previous") == "true"
	offsetFrom, err1 := strconv.Atoi(c.Query("offsetFrom"))
	offsetTo, err2 := strconv.Atoi(c.Query("offsetTo"))
	logFilePosition := c.Query("logFilePosition")

	logSelector := log.DefaultSelection
	if err1 == nil && err2 == nil {
		logSelector = &log.Selection{
			ReferencePoint: log.LogLineId{
				LogTimestamp: log.LogTimestamp(refTimestamp),
				LineNum:      refLineNum,
			},
			OffsetFrom:      offsetFrom,
			OffsetTo:        offsetTo,
			LogFilePosition: logFilePosition,
		}
	}

	result, err := GetLogDetails(k8sClient, namespace, podID, containerID, logSelector, usePreviousLogs)
	if err != nil {
		clog.Error("get log details fail: %v", err)
		response.FailReturn(c, errcode.BadRequest(err))
		return
	}
	response.SuccessReturn(c, result)
}

// GetLogDetails returns logs for particular pod and container. When container is null, logs for the first one
// are returned. Previous indicates to read archived logs created by log rotation or container crash
func GetLogDetails(client kubernetes.Interface, namespace, podID string, container string,
	logSelector *log.Selection, usePreviousLogs bool) (*log.LogDetails, error) {
	pod, err := client.CoreV1().Pods(namespace).Get(context.TODO(), podID, metaV1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if len(container) == 0 {
		container = pod.Spec.Containers[0].Name
	}

	logOptions := mapToLogOptions(container, logSelector, usePreviousLogs)
	rawLogs, err := readRawLogs(client, namespace, podID, logOptions)
	if err != nil {
		return nil, err
	}
	details := ConstructLogDetails(podID, rawLogs, container, logSelector)
	return details, nil
}

// Maps the log selection to the corresponding api object
// Read limits are set to avoid out of memory issues
func mapToLogOptions(container string, logSelector *log.Selection, previous bool) *v1.PodLogOptions {
	logOptions := &v1.PodLogOptions{
		Container:  container,
		Follow:     false,
		Previous:   previous,
		Timestamps: true,
	}

	if logSelector.LogFilePosition == log.Beginning {
		logOptions.LimitBytes = &log.ByteReadLimit
	} else {
		logOptions.TailLines = &log.LineReadLimit
	}

	return logOptions
}

// Construct a request for getting the logs for a pod and retrieves the logs.
func readRawLogs(client kubernetes.Interface, namespace, podID string, logOptions *v1.PodLogOptions) (
	string, error) {
	readCloser, err := openStream(client, namespace, podID, logOptions)
	if err != nil {
		return err.Error(), nil
	}

	defer readCloser.Close()

	result, err := ioutil.ReadAll(readCloser)
	if err != nil {
		return "", err
	}

	return string(result), nil
}

func openStream(client kubernetes.Interface, namespace, podID string, logOptions *v1.PodLogOptions) (io.ReadCloser, error) {
	return client.CoreV1().RESTClient().Get().
		Namespace(namespace).
		Name(podID).
		Resource("pods").
		SubResource("log").
		VersionedParams(logOptions, scheme.ParameterCodec).Stream(context.TODO())
}

// ConstructLogDetails creates a new log details structure for given parameters.
func ConstructLogDetails(podID string, rawLogs string, container string, logSelector *log.Selection) *log.LogDetails {
	parsedLines := log.ToLogLines(rawLogs)
	logLines, fromDate, toDate, logSelection, lastPage := parsedLines.SelectLogs(logSelector)

	readLimitReached := isReadLimitReached(int64(len(rawLogs)), int64(len(parsedLines)), logSelector.LogFilePosition)
	truncated := readLimitReached && lastPage

	info := log.LogInfo{
		PodName:       podID,
		ContainerName: container,
		FromDate:      fromDate,
		ToDate:        toDate,
		Truncated:     truncated,
	}
	return &log.LogDetails{
		Info:      info,
		Selection: logSelection,
		LogLines:  logLines,
	}
}

// Checks if the amount of log file returned from the apiserver is equal to the read limits
func isReadLimitReached(bytesLoaded int64, linesLoaded int64, logFilePosition string) bool {
	return (logFilePosition == log.Beginning && bytesLoaded >= log.ByteReadLimit) ||
		(logFilePosition == log.End && linesLoaded >= log.LineReadLimit)
}
