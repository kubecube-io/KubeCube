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
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gogf/gf/v2/i18n/gi18n"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubecube-io/kubecube/pkg/utils/constants"
)

type header struct {
	Key   string
	Value string
}

func performRequest(r http.Handler, method, path string, body []byte, headers ...header) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if body != nil {
		req.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	}
	for _, h := range headers {
		req.Header.Add(h.Key, h.Value)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestSendEvent(t *testing.T) {
	e := &Event{
		EventTime:       time.Now().Unix(),
		EventVersion:    "V1",
		SourceIpAddress: "127.0.0.1",
		RequestMethod:   http.MethodPost,
		ResponseStatus:  http.StatusOK,
		Url:             "/api/v1/cube/login",
		UserIdentity:    &UserIdentity{"admin"},
		UserAgent:       "HTTP",
		EventType:       constants.EventTypeUserWrite,
		RequestId:       uuid.New().String(),
	}

	sendEvent(e)
}

func TestGetEventName(t *testing.T) {
	file, err := os.Getwd()
	if err != nil {
		t.Fail()
		return
	}
	file = strings.TrimSuffix(file, "pkg/apiserver/middlewares/audit") + "test/i18n"

	e := &Event{}
	ctx := context.Background()
	enInstance := gi18n.Instance()
	enInstance.SetPath(file)
	enInstance.SetLanguage("en")
	h := Handler{
		enInstance,
		enInstance,
	}

	// check post method
	router1 := gin.New()
	router1.POST("/api/v1/cube/proxy/clusters/:cluster/api/v1/namespaces/:namespace/services", func(c *gin.Context) {
		e = h.handleProxyApi(ctx, c, *e)
		return
	})
	_ = performRequest(router1, http.MethodPost, "/api/v1/cube/proxy/clusters/pivot-cluster/api/v1/namespaces/dev/services", []byte(""))
	if e.EventName != "createService" {
		t.Fail()
	}

	// check put method
	router2 := gin.New()
	router2.PUT("/api/v1/cube/proxy/clusters/:cluster/api/v1/namespaces/:namespace/secrets/:name", func(c *gin.Context) {
		e = h.handleProxyApi(ctx, c, *e)
		return
	})
	_ = performRequest(router2, http.MethodPut, "/api/v1/cube/proxy/clusters/pivot-cluster/api/v1/namespaces/dev/secrets/secretA", []byte(""))
	if e.EventName != "updateSecret" {
		t.Fail()
	}
}

func TestGetParameters(t *testing.T) {
	var param string
	router := gin.New()
	router.GET("/api/v1/cube/proxy/clusters/:cluster/apis/apps/v1/namespaces/:namespace/statefulsets/:name", func(c *gin.Context) {
		param = getParameters(c)
		return
	})
	_ = performRequest(router, http.MethodGet, "/api/v1/cube/proxy/clusters/pivot-cluster/apis/apps/v1/namespaces/dev/statefulsets/stsA", []byte(""), header{"cookie", "Auth:Bearer abcde"})
	fmt.Println(param)
}

func TestGetBodyFromReq(t *testing.T) {
	var body string
	router := gin.New()
	router.POST("/api/v1/cube/proxy/clusters/:cluster/api/v1/namespaces/:namespace/services", func(c *gin.Context) {
		body = getBodyFromReq(c)
		return
	})
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "service-example",
		},
	}
	serviceJson, _ := json.Marshal(service)
	_ = performRequest(router, http.MethodPost, "/api/v1/cube/proxy/clusters/pivot-cluster/api/v1/namespaces/dev/services", serviceJson)
	fmt.Println(body)
}
