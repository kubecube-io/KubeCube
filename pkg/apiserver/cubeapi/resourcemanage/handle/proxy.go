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
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	clusterv1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/conversion"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/ctls"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"github.com/kubecube-io/kubecube/pkg/utils/filter"
	"github.com/kubecube-io/kubecube/pkg/utils/kubeconfig"
	"github.com/kubecube-io/kubecube/pkg/utils/page"
	"github.com/kubecube-io/kubecube/pkg/utils/response"
	"github.com/kubecube-io/kubecube/pkg/utils/selector"
	"github.com/kubecube-io/kubecube/pkg/utils/sort"
)

type ProxyHandler struct {
	// enableConvert means proxy handler will convert resources
	enableConvert bool
	// converter the version converter for doing resources convert
	converter conversion.MultiVersionConverter
}

func NewProxyHandler(enableConvert bool) *ProxyHandler {
	return &ProxyHandler{
		enableConvert: enableConvert,
		converter:     multicluster.NewDefaultMultiVersionConverter(multicluster.Interface()),
	}
}

// tryVersionConvert try to convert url and request body by given target cluster
func (h *ProxyHandler) tryVersionConvert(cluster, url string, req *http.Request) (bool, []byte, string, error) {
	if !h.enableConvert {
		return false, nil, "", nil
	}

	_, isNamespaced, gvr, err := conversion.ParseURL(url)
	if err != nil {
		return false, nil, "", err
	}
	converter, err := h.converter.GetVersionConvert(cluster)
	if err != nil {
		return false, nil, "", err
	}
	greetBack, _, recommendVersion, err := converter.GvrGreeting(gvr)
	if err != nil {
		return false, nil, "", err
	}
	if greetBack == conversion.IsPassThrough {
		// gvr is available in target cluster, we do not need version convert
		clog.Debug("%v is available in target cluster %v pass through", gvr.String(), cluster)
		return false, nil, "", nil
	}
	if greetBack == conversion.IsUnknown {
		clog.Info("%v is not found in converter scheme, pass though to cluster %v", gvr.String(), cluster)
		return false, nil, "", nil
	}
	// convert url according to specified gvr at first
	convertedUrl, err := conversion.ConvertURL(url, &schema.GroupVersionResource{Group: recommendVersion.Group, Version: recommendVersion.Version, Resource: gvr.Resource})
	if err != nil {
		return false, nil, "", err
	}

	// we do not need convert body if request not create and update
	if req.Method != http.MethodPost && req.Method != http.MethodPut {
		return true, nil, convertedUrl, nil
	}

	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return false, nil, "", err
	}
	// decode data into internal version of object
	raw, rawGvr, err := converter.Decode(data, nil, nil)
	if err != nil {
		return false, nil, "", err
	}
	if rawGvr.GroupVersion().String() != gvr.GroupVersion().String() {
		return false, nil, "", fmt.Errorf("gv parse failed with pair(%v~%v)", rawGvr.GroupVersion().String(), gvr.GroupVersion().String())
	}
	// covert internal version object int recommend version object
	out, err := converter.Convert(raw, nil, recommendVersion.GroupVersion())
	if err != nil {
		return false, nil, "", err
	}
	// encode concerted object
	convertedObj, err := converter.Encode(out, recommendVersion.GroupVersion())
	if err != nil {
		return false, nil, "", err
	}

	objMeta, err := meta.Accessor(out)
	if err != nil {
		return false, nil, "", err
	}

	if isNamespaced {
		clog.Info("resource (%v/%v) converted with (%v~%v) when visit cluster %v", objMeta.GetNamespace(), objMeta.GetName(), gvr.String(), recommendVersion.GroupVersion().WithResource(gvr.Resource), cluster)
	} else {
		clog.Info("resource (%v) converted with (%v~%v) when visit cluster %v", objMeta.GetName(), gvr.String(), recommendVersion.GroupVersion().WithResource(gvr.Resource), cluster)
	}

	return true, convertedObj, convertedUrl, nil
}

// ProxyHandle proxy all requests access to k8s, request uri format like below
// api/v1/cube/proxy/clusters/{cluster}/{k8s_url}
func (h *ProxyHandler) ProxyHandle(c *gin.Context) {
	// http request params
	cluster := c.Param("cluster")
	url := c.Param("url")
	filter := parseQueryParams(c)

	c.Request.Header.Set(constants.ImpersonateUserKey, "admin")

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

	_, _, gvr, err := conversion.ParseURL(url)
	if err != nil {
		clog.Error(err.Error())
		response.FailReturn(c, errcode.InternalServerError)
		return
	}

	needConvert, convertedObj, convertedUrl, err := h.tryVersionConvert(cluster, url, c.Request)
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

		if needConvert {
			// replace request body and url if need
			if convertedObj != nil {
				r := bytes.NewReader(convertedObj)
				body := io.NopCloser(r)
				req.Body = body
				req.ContentLength = int64(r.Len())
			}
			req.URL.Path = convertedUrl
		}
	}

	errorHandler := func(resp http.ResponseWriter, req *http.Request, err error) {
		if err != nil {
			clog.Warn("cluster %s url %s proxy fail, %v", cluster, url, err)
			response.FailReturn(c, errcode.ServerErr)
			return
		}
	}

	if needConvert {
		// open response filter convert
		_, _, convertedGvr, err := conversion.ParseURL(convertedUrl)
		if err != nil {
			clog.Error(err.Error())
			response.FailReturn(c, errcode.InternalServerError)
			return
		}

		filter.ConvertedGvr = convertedGvr
		filter.EnableConvert = true
		filter.Converter, _ = h.converter.GetVersionConvert(cluster)
		filter.RawGvr = gvr
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
func FilterToMap(c *gin.Context, result []byte) filter.K8sJson {
	resources := parseQueryParams(c)
	return resources.FilterResultToMap(result)
}

// parse request params, include selector, sort and page
func parseQueryParams(c *gin.Context) filter.Filter {
	exact, fuzzy := selector.ParseSelector(c.Query("selector"))
	limit, offset := page.ParsePage(c.Query("pageSize"), c.Query("pageNum"))
	sortName, sortOrder, sortFunc := sort.ParseSort(c.Query("sortName"), c.Query("sortOrder"), c.Query("sortFunc"))

	filter := filter.Filter{
		EnableFilter: needFilter(c),
		Exact:        exact,
		Fuzzy:        fuzzy,
		Limit:        limit,
		Offset:       offset,
		SortName:     sortName,
		SortOrder:    sortOrder,
		SortFunc:     sortFunc,
	}

	return filter
}

func needFilter(c *gin.Context) bool {
	return c.Query("selector")+c.Query("pageSize")+c.Query("pageNum")+
		c.Query("sortName")+c.Query("sortOrder")+c.Query("sortFunc") != ""
}
