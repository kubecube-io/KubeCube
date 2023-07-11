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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

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
		if !withinWhiteList(c.Request.URL, c.Request.Method, auditWhiteList) &&
			!withinWhiteList(c.Request.URL, c.Request.Method, auth.AuthWhiteList) && c.Request.Method != http.MethodGet {
			// can not get resource name from url when create resource, so handle specially
			if c.Request.Method == http.MethodPost && isProxyApi(c.Request.RequestURI) {
				c.Set(constants.EventObjectName, getPostObjectName(c))
			}
			w := &responseBodyWriter{body: &bytes.Buffer{}, ResponseWriter: c.Writer}
			c.Writer = w

			c.Next()

			go h.handle(c, w)
		} else {
			c.Next()
		}
	}

}

func (h *Handler) handle(c *gin.Context, w *responseBodyWriter) {
	e := &Event{
		EventTime:         time.Now().UnixNano() / int64(time.Millisecond),
		EventVersion:      "V1",
		SourceIpAddress:   c.ClientIP(),
		RequestMethod:     c.Request.Method,
		ResponseStatus:    c.Writer.Status(),
		Url:               c.Request.URL.String(),
		UserAgent:         "HTTP",
		RequestParameters: ConsistParameters(c, nil),
		EventType:         constants.EventTypeUserWrite,
		RequestId:         uuid.New().String(),
		ResponseElements:  w.body.String(),
		EventSource:       env.AuditEventSource(),
		UserIdentity:      getUserIdentity(c),
	}

	// get response
	resp, isExist := c.Get(constants.EventRespBody)
	if isExist == true {
		e.ResponseElements = resp.(string)
	}

	// if request failed, set errCode and ErrorMessage
	if e.ResponseStatus < 200 || e.ResponseStatus > 299 {
		e.ErrorCode = strconv.Itoa(c.Writer.Status())
		e.ErrorMessage = e.ResponseElements
	}

	// get event name and description

	ctx := context.Background()
	eventName, isExist := c.Get(constants.EventName)
	if isExist == true {
		e.EventName = eventName.(string)
		e.Description = h.Translate(ctx, eventName.(string))
		e.ResourceReports = []Resource{{
			ResourceType: h.Translate(ctx, c.GetString(audit.EventResourceType)),
			ResourceName: c.GetString(audit.EventResourceName),
		}}
	} else {
		e = h.handleProxyApi(ctx, c, *e)
	}

	if e.EventName != "" {
		sendEvent(e)
	}
}

func (h *Handler) Translate(ctx context.Context, content string) string {
	t := h.EnvInstance
	if t == nil {
		t = h.EnInstance
	}
	return t.Translate(ctx, content)
}

type Options struct {
	Translate bool
}

func (h *Handler) SendEvent(c *gin.Context, event *Event, options *Options) {
	if event == nil {
		return
	}

	if len(event.EventName) == 0 || len(event.RequestMethod) == 0 || event.ResponseStatus == 0 || len(event.Url) == 0 {
		clog.Warn("missing required field of event: %+v", event)
		return
	}

	// set default value if unset
	if event.EventTime == 0 {
		event.EventTime = time.Now().UnixNano() / int64(time.Millisecond)
	}
	if len(event.EventVersion) == 0 {
		event.EventVersion = "V1"
	}
	if len(event.SourceIpAddress) == 0 {
		event.SourceIpAddress = c.ClientIP()
	}
	if len(event.UserAgent) == 0 {
		event.UserAgent = "HTTP"
	}
	if len(event.EventType) == 0 {
		event.EventType = constants.EventTypeUserWrite
	}
	if len(event.RequestId) == 0 {
		event.RequestId = uuid.New().String()
	}
	if len(event.EventSource) == 0 {
		event.EventSource = env.AuditEventSource()
	}
	if event.UserIdentity == nil {
		event.UserIdentity = getUserIdentity(c)
	}

	ctx := context.Background()

	if options.Translate {
		// translate event description
		event.Description = h.Translate(ctx, event.Description)
	}

	for i, v := range event.ResourceReports {
		if len(v.ResourceName) == 0 || len(v.ResourceType) == 0 {
			clog.Warn("missing required field of event: %+v", event)
			return
		}
		if options.Translate {
			// translate resource type
			v.ResourceType = h.Translate(ctx, v.ResourceType)
			event.ResourceReports[i] = v
		}
	}

	// send event as we wish
	sendEvent(event)
}

func sendEvent(e *Event) {
	clog.Debug("[audit] send event to audit service")
	jsonstr, err := json.Marshal(e)
	if err != nil {
		clog.Error("[audit] json marshal event error: %v", err)
		return
	}
	buffer := bytes.NewBuffer(jsonstr)
	clog.Debug("[audit] [%s] [%s] body: %s", auditSvc.Method, auditSvc.URL, string(jsonstr))
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
	if !isProxyApi(requestURI) {
		return &e
	}

	// get object type from url
	queryUrl := strings.TrimPrefix(strings.Split(requestURI, "?")[0], constants.ApiPathRoot)
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

			} else if length >= i+4 {
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
	var objectTypeTitle string
	if len(objectType) > 0 {
		objectTypeTitle = strings.Title(objectType[:len(objectType)-1])
	}
	e.EventName = h.EnInstance.Translate(ctx, method) + objectTypeTitle
	e.Description = h.Translate(ctx, method) + h.Translate(ctx, objectType)

	if http.MethodPost == method && objectName == "" {
		objectName = c.GetString(constants.EventObjectName)
	}
	e.ResourceReports = []Resource{{
		ResourceType: objectType[:len(objectType)-1],
		ResourceName: objectName,
	}}
	return &e
}

func isProxyApi(requestURI string) bool {
	if strings.HasPrefix(requestURI, constants.ApiPathRoot+"/proxy") {
		return true
	}
	return false
}

// get user name from token
func getUserIdentity(c *gin.Context) *UserIdentity {
	userIdentity := &UserIdentity{}
	accountId := c.GetString(constants.EventAccountId)
	if len(accountId) > 0 {
		userIdentity.AccountId = accountId
		return userIdentity
	} else {
		userName := c.GetString(constants.UserName)
		if len(userName) > 0 {
			userIdentity.AccountId = userName
			return userIdentity
		}
	}

	userInfo, err := token.GetUserFromReq(c.Request)
	if err != nil {
		return nil
	}
	userIdentity.AccountId = userInfo.Username
	return userIdentity
}

// ConsistParameters includes header、body and queryString
func ConsistParameters(c *gin.Context, body []byte) string {
	var parameters parameters

	auditHeaders := auditSvc.AuditHeaders
	headers := make(map[string][]string)
	for _, h := range auditHeaders {
		for k, v := range c.Request.Header {
			if h == k {
				headers[k] = v
				break
			}
		}
	}
	parameters.Headers = headers
	parameters.Body = c.GetString(audit.EventRequestBody)
	if body != nil {
		parameters.Body = string(body)
	}

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
	if err != nil {
		clog.Warn("[audit] get raw data err: %s", err.Error())
		return ""
	}
	// put request body back
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	obj := unstructured.Unstructured{}
	if err = json.Unmarshal(data, &obj); err != nil {
		clog.Warn("[audit] unmarshal request body err: %s", err.Error())
		return ""
	}

	return obj.GetName()
}
