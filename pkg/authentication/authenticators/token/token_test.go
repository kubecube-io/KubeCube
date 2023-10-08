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

package token

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type header struct {
	Key   string
	Value string
}

func performRequest(r http.Handler, method, path string, body []byte, headers []header, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if body != nil {
		req.Body = io.NopCloser(bytes.NewBuffer(body))
	}
	for _, h := range headers {
		req.Header.Add(h.Key, h.Value)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestGetTokenFromReq(t *testing.T) {
	var token string
	router := gin.New()
	router.GET("/api/v1/cube/proxy/clusters/:cluster/apis/apps/v1/namespaces/:namespace/statefulsets/:name", func(c *gin.Context) {
		token, _ = GetTokenFromReq(c.Request)
		return
	})
	_ = performRequest(router, http.MethodGet, "/api/v1/cube/proxy/clusters/pivot-cluster/apis/apps/v1/namespaces/dev/statefulsets/stsA", []byte(""), []header{{"Authorization", "Bearer abcde"}})
	if token != "abcde" {
		t.Fail()
	}
	cookie := &http.Cookie{Name: "Authorization", Value: "Bearer abcde"}
	_ = performRequest(router, http.MethodGet, "/api/v1/cube/proxy/clusters/pivot-cluster/apis/apps/v1/namespaces/dev/statefulsets/stsA", []byte(""), []header{}, cookie)
	if token != "abcde" {
		t.Fail()
	}
}
