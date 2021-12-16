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

package audit

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	jsoniter "github.com/json-iterator/go"

	"github.com/kubecube-io/kubecube/pkg/apiserver/middlewares/auth"
	"github.com/kubecube-io/kubecube/pkg/authentication/authenticators/token"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/audit"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/env"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

const (
	eventActionCreate = "create"
	eventActionUpdate = "update"
	eventActionDelete = "delete"
	eventActionQuery  = "query"

	eventRespBody = "responseBody"
)

var (
	auditWhiteList = map[string]string{
		constants.ApiPathRoot + "/audit": "POST",
	}
	auditSvc env.AuditSvcApi
)

func init() {
	auditSvc = env.AuditSVC()
}

func withinWhiteList(url *url.URL, method string, whiteList map[string]string) bool {
	queryUrl := url.Path
	for k, v := range whiteList {
		match, err := regexp.MatchString(k, queryUrl)
		if err == nil && match && method == v {
			return true
		}
	}
	return false
}

func Audit() gin.HandlerFunc {
	return func(c *gin.Context) {

		w := &responseBodyWriter{body: &bytes.Buffer{}, ResponseWriter: c.Writer}
		c.Writer = w
		c.Next()
		if !withinWhiteList(c.Request.URL, c.Request.Method, auditWhiteList) &&
			!withinWhiteList(c.Request.URL, c.Request.Method, auth.AuthWhiteList) && c.Request.Method != http.MethodGet {
			clog.Debug("[audit] get event information")
			e := &Event{
				EventTime:         time.Now().UnixNano() / int64(time.Millisecond),
				EventVersion:      "V1",
				SourceIpAddress:   c.ClientIP(),
				RequestMethod:     c.Request.Method,
				ResponseStatus:    c.Writer.Status(),
				Url:               c.Request.URL.String(),
				UserAgent:         "HTTP",
				RequestParameters: getParameters(c),
				EventType:         audit.EventTypeUserWrite,
				RequestId:         uuid.New().String(),
				ResponseElements:  w.body.String(),
				EventSource:       env.AuditEventSource(),
				UserIdentity:      getUserIdentity(c),
			}

			// get response
			resp, isExist := c.Get(eventRespBody)
			if isExist == true {
				e.ResponseElements = resp.(string)
			}

			// if request failed, set errCode and ErrorMessage
			if e.ResponseStatus != http.StatusOK {
				e.ErrorCode = strconv.Itoa(c.Writer.Status())
				e.ErrorMessage = e.ResponseElements
			}

			// get event name and description
			eventName, isExist := c.Get(audit.EventName)
			if isExist == true {
				e.EventName = eventName.(string)
				e.Description = c.GetString(audit.EventDescription)
				e.ResourceReports = []Resource{{
					ResourceType: c.GetString(audit.EventDescription),
					ResourceName: c.GetString(audit.EventResourceName),
				}}
			} else {
				e = getEventName(c, *e)
			}

			go sendEvent(e)
		}
	}

}

func sendEvent(e *Event) {
	clog.Debug("[audit] send event to audit service")
	jsonstr, err := json.Marshal(e)
	if err != nil {
		clog.Error("[audit] json marshal event error: %v", err)
		return
	}
	buffer := bytes.NewBuffer(jsonstr)
	request, err := http.NewRequest(auditSvc.Method, auditSvc.URL, buffer)
	if err != nil {
		clog.Error("[audit] create http request error: %v", err)
		return
	}
	headers := strings.Split(auditSvc.Header, ";")
	for _, header := range headers {
		kv := strings.Split(header, "=")
		if len(kv) != 2 {
			continue
		}
		request.Header.Set(kv[0], kv[1])
	}
	client := http.Client{}
	resp, err := client.Do(request.WithContext(context.TODO()))
	if err != nil {
		clog.Error("[audit] client.Do error: %s", err)
		return
	}
	defer resp.Body.Close()

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		clog.Error("[audit] read response body error: %s", err)
		return
	}
	clog.Debug("[audit] response is %s", string(respBytes))
	return
}

// get event name and description
func getEventName(c *gin.Context, e Event) *Event {
	var (
		methodStr  string
		objectType string
		objectName string
	)

	requestURI := c.Request.RequestURI
	method := c.Request.Method

	switch method {
	case http.MethodPost:
		methodStr = eventActionCreate
		e.Description = "创建"
		break
	case http.MethodPut, http.MethodPatch:
		methodStr = eventActionUpdate
		e.Description = "更新"
		break
	case http.MethodDelete:
		methodStr = eventActionDelete
		e.Description = "删除"
	}

	queryUrl := strings.Trim(strings.Split(fmt.Sprint(requestURI), "?")[0], "/api/v1/cube")
	urlstrs := strings.Split(queryUrl, "/")
	length := len(urlstrs)
	for i, str := range urlstrs {
		if str == constants.K8sResourceNamespace {
			if length == i+1 {
				objectType = constants.K8sResourceNamespace
			} else if length == i+2 {
				objectType = constants.K8sResourceNamespace
				objectName = urlstrs[i+1]

			} else if length == i+3 {
				objectType = urlstrs[i+2]

			} else if length == i+4 {
				objectType = urlstrs[i+2]
				objectName = urlstrs[i+3]
			}
			break
		}

		if str == constants.K8sResourceVersion && urlstrs[i+1] != constants.K8sResourceNamespace {
			objectType = urlstrs[i+1]
			if i+2 < len(urlstrs) {
				objectName = urlstrs[i+2]
			}
			break
		}
	}

	e.EventName = methodStr + objectType
	e.Description = e.Description + audit.ResourceType[objectType]
	e.ResourceReports = []Resource{{
		ResourceType: objectType[:len(objectType)-1],
		ResourceName: objectName,
	}}
	return &e
}

// get user name from token
func getUserIdentity(c *gin.Context) *UserIdentity {

	userIdentity := &UserIdentity{}
	accountId, isExist := c.Get(audit.EventAccountId)
	if isExist {
		userIdentity.AccountId = accountId.(string)
		return userIdentity
	}

	userInfo, err := token.GetUserFromReq(c.Request)
	if err != nil {
		return nil
	}
	userIdentity.AccountId = userInfo.Username
	return userIdentity
}

// get request params, includes header、body and queryString
func getParameters(c *gin.Context) string {
	var parameters parameters

	parameters.Headers = c.Request.Header.Clone()
	parameters.Body = getBodyFromReq(c)

	query := make(map[string]string)
	params := c.Params
	for _, param := range params {
		query[param.Key] = param.Value
	}
	parameters.Query = query
	paramJson, err := json.Marshal(parameters)
	if err != nil {
		clog.Error("marshal param error: %s", err)
		return ""
	}
	return string(paramJson)

}

type parameters struct {
	Query   map[string]string   `json:"querystring"`
	Body    string              `json:"body"`
	Headers map[string][]string `json:"header"`
}

func getBodyFromReq(c *gin.Context) string {
	switch c.Request.Method {
	case http.MethodPatch:
	case http.MethodPut:
	case http.MethodPost:
		data, _ := ioutil.ReadAll(c.Request.Body)
		return string(data)
	}
	return ""
}

type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r responseBodyWriter) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func (r responseBodyWriter) WriteString(s string) (n int, err error) {
	r.body.WriteString(s)
	return r.ResponseWriter.WriteString(s)
}
