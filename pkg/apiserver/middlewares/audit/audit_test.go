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
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gogf/gf/v2/i18n/gi18n"
	"github.com/google/uuid"
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
