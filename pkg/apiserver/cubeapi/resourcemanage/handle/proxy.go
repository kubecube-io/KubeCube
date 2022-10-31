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
	"fmt"
	"io"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/runtime"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/conversion"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"github.com/kubecube-io/kubecube/pkg/utils/filter"
	"github.com/kubecube-io/kubecube/pkg/utils/page"
	requestutil "github.com/kubecube-io/kubecube/pkg/utils/request"
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
		// we just record error and pass through anyway
		clog.Warn(err.Error())
	}
	if greetBack != conversion.IsNeedConvert {
		// pass through anyway if not need convert
		clog.Info("%v greet cluster %v is %v, pass through", gvr.String(), cluster, greetBack)
		return false, nil, "", nil
	}
	if recommendVersion == nil {
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
	proxyUrl := c.Param("url")
	filter := parseQueryParams(c)

	username := c.GetString(constants.UserName)
	if len(username) == 0 {
		clog.Warn("username is empty")
	}

	// fixme: use correct user
	c.Request.Header.Set(constants.ImpersonateUserKey, "admin")
	internalCluster, err := multicluster.Interface().Get(cluster)
	if err != nil {
		clog.Error(err.Error())
		response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, err.Error()))
		return
	}
	transport, err := multicluster.Interface().GetTransport(cluster)
	if err != nil {
		clog.Error(err.Error())
		response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, err.Error()))
		return
	}
	_, _, gvr, err := conversion.ParseURL(proxyUrl)
	if err != nil {
		clog.Error(err.Error())
		response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, err.Error()))
		return
	}

	needConvert, convertedObj, convertedUrl, err := h.tryVersionConvert(cluster, proxyUrl, c.Request)
	if err != nil {
		clog.Error(err.Error())
		response.FailReturn(c, errcode.InternalServerError)
		return
	}

	// todo: open it when impersonate fixed.
	//allowed, err := belongs.RelationshipDetermine(context.Background(), internalCluster.Client, proxyUrl, username)
	//if err != nil {
	//	clog.Warn(err.Error())
	//} else if !allowed {
	//	response.FailReturn(c, errcode.ForbiddenErr)
	//	return
	//}

	// create director
	director := func(req *http.Request) {
		labelSelector := selector.ParseLabelSelector(c.Query("selector"))

		uri, err := url.ParseRequestURI(internalCluster.Config.Host)
		if err != nil {
			clog.Error("Could not parse host, host: %s , err: %v", internalCluster.Config.Host, err)
			response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, "Could not parse host, host: %s , err: %v", internalCluster.Config.Host, err))
		}
		uri.RawQuery = c.Request.URL.RawQuery
		uri.Path = proxyUrl
		req.URL = uri
		req.Host = internalCluster.Config.Host

		err = requestutil.AddFieldManager(req, username)
		if err != nil {
			clog.Error("fail to add fieldManager due to %s", err.Error())
		}
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

		//In order to improve processing efficiency
		//this method converts requests starting with metadata.labels in the selector into k8s labelSelector requests
		// todo This method can be further optimized and extracted as a function to improve readability
		if len(labelSelector) > 0 {
			labelSelectorQueryString := ""
			// Take out the query value in the selector and stitch it into the query field of labelSelector
			// for example: selector=metadata.labels.key=value1|value2|value3
			// then it should be converted to: key+in+(value1,value2,value3)
			for key, value := range labelSelector {
				if len(value) < 1 {
					continue
				}
				labelSelectorQueryString += key
				labelSelectorQueryString += "+in+("
				labelSelectorQueryString += strings.Join(value, ",")
				labelSelectorQueryString += ")"
				labelSelectorQueryString += ","
			}
			if len(labelSelectorQueryString) > 0 {
				labelSelectorQueryString = strings.TrimRight(labelSelectorQueryString, ",")
			}
			labelSelectorQueryString = url.PathEscape(labelSelectorQueryString)
			// Old query parameters may have the following conditions:
			// empty
			// has selector: selector=key=value
			// has selector and labelSelector: selector=key=value&labelSelector=key=value
			// has selector and labelSelector and others: selector=key=value&labelSelector=key=value&fieldSelector=key=value
			// so, use & to split it
			queryArray := strings.Split(req.URL.RawQuery, "&")
			queryString := ""
			labelSelectorSet := false
			for _, v := range queryArray {
				//if it start with labelSelector=, then append converted labelSelector string
				if strings.HasPrefix(v, "labelSelector=") {
					queryString += v + "," + labelSelectorQueryString
					labelSelectorSet = true
					// else if url like: selector=key=value&labelSelector, then use converted labelSelector string replace it
				} else if strings.HasPrefix(v, "labelSelector") {
					queryString += "labelSelector=" + labelSelectorQueryString
					labelSelectorSet = true
					// else no need to do this
				} else {
					queryString += v
				}
				queryString += "&"
			}
			// If the query parameter does not exist labelSelector
			// append converted labelSelector string
			if len(queryString) > 0 && labelSelectorSet == false {
				queryString += "&labelSelector=" + labelSelectorQueryString
			}
			req.URL.RawQuery = queryString
		}
	}

	errorHandler := func(resp http.ResponseWriter, req *http.Request, err error) {
		if err != nil {
			clog.Warn("cluster %s url %s proxy fail, %v", cluster, proxyUrl, err)
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

	requestProxy := &httputil.ReverseProxy{Director: director, Transport: transport, ModifyResponse: filter.ModifyResponse, ErrorHandler: errorHandler}

	// trim auth token here
	c.Request.Header.Del(constants.AuthorizationHeader)

	requestProxy.ServeHTTP(c.Writer, c.Request)
}

// product match/sort/page to other function
func Filter(c *gin.Context, object runtime.Object) (*int, error) {
	resources := parseQueryParams(c)
	total, err := resources.FilterObjectList(object)
	if err != nil {
		clog.Error("filter userList error, err: %s", err.Error())
		return nil, err
	}
	return total, nil
}

// parse request params, include selector, sort and page
func parseQueryParams(c *gin.Context) *filter.Filter {
	exact, fuzzy := selector.ParseSelector(c.Query("selector"))
	limit, offset := page.ParsePage(c.Query("pageSize"), c.Query("pageNum"))
	sortName, sortOrder, sortFunc := sort.ParseSort(c.Query("sortName"), c.Query("sortOrder"), c.Query("sortFunc"))

	// fixme use struct
	return filter.NewFilter(exact, fuzzy, limit, offset, sortName, sortOrder, sortFunc, nil)
}
