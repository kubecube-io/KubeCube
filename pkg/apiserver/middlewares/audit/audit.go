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
	"github.com/gogf/gf/v2/i18n/gi18n"
	"github.com/google/uuid"
	jsoniter "github.com/json-iterator/go"

	"github.com/kubecube-io/kubecube/pkg/apiserver/middlewares/auth"
	"github.com/kubecube-io/kubecube/pkg/authentication/authenticators/token"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/audit"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/env"
	"github.com/kubecube-io/kubecube/pkg/utils/international"
)

var (
	json           = jsoniter.ConfigCompatibleWithStandardLibrary
	auditWhiteList = map[string]string{
		constants.ApiPathRoot + "/audit": "POST",
	}
	auditSvc env.AuditSvcApi
)

const (
	eventRespBody   = "responseBody"
	eventObjectName = "objectName"
)

type Handler struct {
	EnInstance  *gi18n.Manager
	EnvInstance *gi18n.Manager
}

func init() {
	auditSvc = env.AuditSVC()
}

func NewHandler(managers *international.Gi18nManagers) Handler {
	enT := managers.GetInstants("en")
	envT := managers.GetInstants(env.AuditLanguage())
	h := Handler{enT, envT}
	return h
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

func (h *Handler) Audit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// can not get resource name from url when create resource, so handle specially
		if c.Request.Method == http.MethodPost {
			c.Set(eventObjectName, getPostObjectName(c))
		}
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
				EventType:         constants.EventTypeUserWrite,
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
			t := h.EnvInstance
			if t == nil {
				t = h.EnInstance
			}
			ctx := context.Background()
			eventName, isExist := c.Get(constants.EventName)
			if isExist == true {
				e.EventName = eventName.(string)
				e.Description = t.Translate(ctx, eventName.(string))
				e.ResourceReports = []Resource{{
					ResourceType: t.Translate(ctx, c.GetString(audit.EventResourceType)),
					ResourceName: c.GetString(audit.EventResourceName),
				}}
			} else {
				e = h.handleProxyApi(ctx, c, *e)
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
func (h *Handler) handleProxyApi(ctx context.Context, c *gin.Context, e Event) *Event {
	var (
		objectType string
		objectName string
	)

	requestURI := c.Request.RequestURI
	if !strings.HasPrefix(requestURI, "/api/v1/cube/proxy") &&
		!strings.HasPrefix(requestURI, "/api/v1/cube/extend") {
		return &e
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

	method := c.Request.Method
	e.EventName = h.EnInstance.Translate(ctx, method) + strings.Title(objectType[:len(objectType)-1])
	t := h.EnvInstance
	if t == nil {
		t = h.EnInstance
	}
	e.Description = t.Translate(ctx, method) + t.Translate(ctx, objectType)

	if http.MethodPost == method && objectName == "" {
		objectName = c.GetString(eventObjectName)
	}
	e.ResourceReports = []Resource{{
		ResourceType: objectType[:len(objectType)-1],
		ResourceName: objectName,
	}}
	return &e
}

// get user name from token
func getUserIdentity(c *gin.Context) *UserIdentity {

	userIdentity := &UserIdentity{}
	accountId, isExist := c.Get(constants.EventAccountId)
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

func getPostObjectName(c *gin.Context) string {
	// get request body
	data, err := c.GetRawData()
	// put request body back
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))
	// get resource name from metadata.name
	body := make(map[string]interface{})
	err = json.Unmarshal(data, &body)
	if err != nil {
		clog.Error("[audit] unmarshal request body err: %s", err.Error())
		return ""
	}
	metadata, exist := body["metadata"]
	if !exist {
		clog.Error("[audit] the resource metadata is nil")
		return ""
	}
	metadataMap, ok := metadata.(map[string]interface{})
	if !ok {
		clog.Error("[audit] convert metadata to map failed")
		return ""
	}
	name, exist := metadataMap["name"]
	if !exist {
		clog.Error("[audit] the resource metadata.name is nil")
		return ""
	}
	nameStr, ok := name.(string)
	if !ok {
		clog.Error("[audit] convert name to string failed")
		return ""
	}
	return nameStr
}
