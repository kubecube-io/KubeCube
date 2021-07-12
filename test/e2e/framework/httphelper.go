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
package framework

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type HttpHelper struct {
	HostPath     string
	Admin        string
	TenantAdmin  string
	ProjectAdmin string
	User         string
	Client       http.Client
	Cookies      []*http.Cookie
}

func NewHttpHelper() *HttpHelper {
	hostPath := viper.GetString("kubecube.hostPath")
	admin := viper.GetString("kubecube.multiuser.admin")
	tenantAdmin := viper.GetString("kubecube.multiuser.tenantAdmin")
	projectAdmin := viper.GetString("kubecube.multiuser.projectAdmin")
	user := viper.GetString("kubecube.multiuser.user")

	h := &HttpHelper{
		HostPath:     hostPath,
		Admin:        admin,
		TenantAdmin:  tenantAdmin,
		ProjectAdmin: projectAdmin,
		User:         user,
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	h.Client = http.Client{
		Transport: tr,
		Timeout:   30 * time.Second,
	}

	return h
}

func (h *HttpHelper) Login() *HttpHelper {
	return h.LoginByUser(h.Admin)
}

func (h *HttpHelper) LoginByUser(username string) *HttpHelper {
	body := map[string]string{
		"name":      username,
		"password":  username,
		"loginType": "normal",
	}
	bodyjson, _ := json.Marshal(body)
	req := h.Request("POST", h.FormatUrl("/login"), string(bodyjson))
	resp, err := h.Client.Do(&req)
	if err != nil {
		return h
	}
	h.Cookies = resp.Cookies()
	return h
}

func (h *HttpHelper) FormatUrl(url string) string {
	return fmt.Sprintf("%s%s%s", h.HostPath, "/api/v1/cube", url)
}

func (h *HttpHelper) Request(method, urlVal, data string) http.Request {
	var req *http.Request

	if data == "" {
		urlArr := strings.Split(urlVal, "?")
		if len(urlArr) == 2 {
			urlVal = urlArr[0] + "?" + url.PathEscape(urlArr[1])
		}
		req, _ = http.NewRequest(method, urlVal, nil)
	} else {
		req, _ = http.NewRequest(method, urlVal, strings.NewReader(data))
	}

	for _, c := range h.Cookies {
		req.AddCookie(c)
	}

	req.Header.Add("Content-Type", "application/json")

	return *req
}

type MultiRequestResponse struct {
	Resp *http.Response
	Err  error
}

// multi user request test
func (h *HttpHelper) MultiUserRequest(method, url, body string) map[string]MultiRequestResponse {
	ret := make(map[string]MultiRequestResponse)
	h.LoginByUser(h.Admin)
	r1 := h.Request(method, h.FormatUrl(url), body)
	resp1, err1 := h.Client.Do(&r1)
	ret["admin"] = MultiRequestResponse{resp1, err1}

	h.LoginByUser(h.TenantAdmin)
	r2 := h.Request(method, h.FormatUrl(url), body)
	resp2, err2 := h.Client.Do(&r2)
	ret["tenantAdmin"] = MultiRequestResponse{resp2, err2}

	h.LoginByUser(h.ProjectAdmin)
	r3 := h.Request(method, h.FormatUrl(url), body)
	resp3, err3 := h.Client.Do(&r3)
	ret["projectAdmin"] = MultiRequestResponse{resp3, err3}

	h.LoginByUser(h.User)
	r4 := h.Request(method, h.FormatUrl(url), body)
	resp4, err4 := h.Client.Do(&r4)
	ret["user"] = MultiRequestResponse{resp4, err4}

	return ret
}
