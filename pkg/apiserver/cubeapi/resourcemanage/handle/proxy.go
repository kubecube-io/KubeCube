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
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/types"

	clusterv1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/ctls"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"github.com/kubecube-io/kubecube/pkg/utils/kubeconfig"
	"github.com/kubecube-io/kubecube/pkg/utils/response"
)

// ProxyHandle proxy all requests access to k8s, request uri format like below
// api/v1/cube/proxy/clusters/{cluster}/{k8s_url}
func ProxyHandle(c *gin.Context) {
	// http request params
	cluster := c.Param("cluster")
	url := c.Param("url")
	filter := parseQueryParams(c)
	isFilter := c.Query("isFilter")

	// get cluster info by cluster name
	host, certData, keyData, caData := getClusterInfo(cluster)
	if host == "" {
		response.FailReturn(c, errcode.ClusterNotFoundError(cluster))
		return
	}

	ts, err := ctls.MakeMTlsTransportByPem(caData, certData, keyData)
	if err != nil {
		clog.Error(err.Error())
		response.FailReturn(c, errcode.InternalServerError)
		return
	}

	// create director
	director := func(req *http.Request) {
		req.URL.Scheme = "https"
		req.URL.Host = host
		req.Host = host
		req.URL.Path = url
	}

	errorHandler := func(resp http.ResponseWriter, req *http.Request, err error) {
		if err != nil {
			clog.Warn("cluster %s url %s proxy fail, %v", cluster, url, err)
			response.FailReturn(c, errcode.ServerErr)
			return
		}
	}
	if isFilter == "false" {
		requestProxy := &httputil.ReverseProxy{Director: director, Transport: ts, ErrorHandler: errorHandler}
		requestProxy.ServeHTTP(c.Writer, c.Request)
		return
	}
	requestProxy := &httputil.ReverseProxy{Director: director, Transport: ts, ModifyResponse: filter.ModifyResponse, ErrorHandler: errorHandler}

	// trim auth token here
	c.Request.Header.Del(constants.AuthorizationHeader)

	requestProxy.ServeHTTP(c.Writer, c.Request)
}

// get cluster info by clusterName
func getClusterInfo(clusterName string) (string, []byte, []byte, []byte) {

	client := clients.Interface().Kubernetes(constants.LocalCluster)
	if client == nil {
		return "", nil, nil, nil
	}
	clusterInfo := clusterv1.Cluster{}
	err := client.Cache().Get(context.Background(), types.NamespacedName{Name: clusterName}, &clusterInfo)
	if err != nil {
		clog.Info("the cluster %s is no exist: %v", clusterName, err)
		return "", nil, nil, nil
	}

	host := clusterInfo.Spec.KubernetesAPIEndpoint
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")

	config, err := kubeconfig.LoadKubeConfigFromBytes(clusterInfo.Spec.KubeConfig)
	if err != nil {
		clog.Info("the cluster %s parser kubeconfig fail: %v", clusterName, err)
		return "", nil, nil, nil
	}

	return host, config.CertData, config.KeyData, config.CAData
}

// product match/sort/page to other function
func Filter(c *gin.Context, result []byte) []byte {
	resources := parseQueryParams(c)
	return resources.FilterResult(result)
}

// product match/sort/page to other function
func FilterToMap(c *gin.Context, result []byte) resources.K8sJson {
	resources := parseQueryParams(c)
	return resources.FilterResultToMap(result)
}

// parse request params, include selector, sort and page
func parseQueryParams(c *gin.Context) resources.Filter {

	exact, fuzzy := parseSelector(c.Query("selector"))
	limit, offset := parsePage(c.Query("pageSize"), c.Query("pageNum"))
	sortName, sortOrder, sortFunc := parseSort(c.Query("sortName"), c.Query("sortOrder"), c.Query("sortFunc"))

	filter := resources.Filter{
		Exact:     exact,
		Fuzzy:     fuzzy,
		Limit:     limit,
		Offset:    offset,
		SortName:  sortName,
		SortOrder: sortOrder,
		SortFunc:  sortFunc,
	}
	return filter
}

// filter selector
// exact query：selector=key1=value1,key2=value2,key3=value3
// fuzzy query：selector=key1~value1,key2~value2,key3~value3
// support mixed query：selector=key1~value1,key2=value2,key3=value3
func parseSelector(selectorStr string) (exact, fuzzy map[string]string) {
	if selectorStr == "" {
		return nil, nil
	}

	exact = make(map[string]string, 0)
	fuzzy = make(map[string]string, 0)

	labels := strings.Split(selectorStr, ",")
	for _, label := range labels {
		if i := strings.IndexAny(label, "~="); i > 0 {
			if label[i] == '=' {
				exact[label[:i]] = label[i+1:]
			} else {
				fuzzy[label[:i]] = label[i+1:]
			}
		}
	}

	return
}

// page=10,1, means limit=10&page=1, default 10,1
// offset=(page-1)*limit
func parsePage(pageSize string, pageNum string) (limit, offset int) {
	limit = 10
	offset = 0

	limit, err := strconv.Atoi(pageSize)
	if err != nil {
		limit = 10
	}

	page, err := strconv.Atoi(pageNum)
	if err != nil || page < 1 {
		offset = 0
	} else {
		offset = (page - 1) * limit
	}

	return
}

// sortName=creationTimestamp, sortOrder=asc
func parseSort(name string, order string, sFunc string) (sortName, sortOrder, sortFunc string) {
	sortName = "metadata.name"
	sortOrder = "asc"
	sortFunc = "string"

	if name == "" {
		return
	}
	sortName = name

	if strings.EqualFold(order, "desc") {
		sortOrder = "desc"
	}

	if sFunc != "" {
		sortFunc = sFunc
	}

	return
}
