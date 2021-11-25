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
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/spf13/viper"
)

type AuthUser struct {
	Username string
	Ak       string
	Sk       string
	Token    string
	Cookies  []*http.Cookie
	AuthType int // 0 cookie, 1 token
}

type HttpHelper struct {
	HostPath     string
	Admin        AuthUser
	TenantAdmin  AuthUser
	ProjectAdmin AuthUser
	User         AuthUser
	Client       http.Client
}

var httphelper *HttpHelper

// single mode
func NewSingleHttpHelper() *HttpHelper {
	if httphelper != nil {
		return httphelper
	}

	httphelper = NewHttpHelper().AuthToken()
	return httphelper
}

func NewHttpHelper() *HttpHelper {
	hostPath := viper.GetString("kubecube.hostPath")
	admin := viper.GetString("kubecube.multiuser.admin")
	tenantAdmin := viper.GetString("kubecube.multiuser.tenantAdmin")
	projectAdmin := viper.GetString("kubecube.multiuser.projectAdmin")
	user := viper.GetString("kubecube.multiuser.user")
	adminAk := viper.GetString("kubecube.multiuserAkSk.adminAk")
	tenantAdminAk := viper.GetString("kubecube.multiuserAkSk.tenantAdminAk")
	projectAdminAk := viper.GetString("kubecube.multiuserAkSk.projectAdminAk")
	userAk := viper.GetString("kubecube.multiuserAkSk.userAk")
	adminSk := viper.GetString("kubecube.multiuserAkSk.adminSk")
	tenantAdminSk := viper.GetString("kubecube.multiuserAkSk.tenantAdminSk")
	projectAdminSk := viper.GetString("kubecube.multiuserAkSk.projectAdminSk")
	userSk := viper.GetString("kubecube.multiuserAkSk.userSk")

	h := &HttpHelper{
		HostPath:     hostPath,
		Admin:        AuthUser{Username: admin, Ak: adminAk, Sk: adminSk},
		TenantAdmin:  AuthUser{Username: tenantAdmin, Ak: tenantAdminAk, Sk: tenantAdminSk},
		ProjectAdmin: AuthUser{Username: projectAdmin, Ak: projectAdminAk, Sk: projectAdminSk},
		User:         AuthUser{Username: user, Ak: userAk, Sk: userSk},
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
	h.LoginByUser(&h.Admin)
	h.LoginByUser(&h.TenantAdmin)
	h.LoginByUser(&h.ProjectAdmin)
	h.LoginByUser(&h.User)
	return h
}

func (h *HttpHelper) LoginByUser(user *AuthUser) error {
	body := map[string]string{
		"name":      user.Username,
		"password":  user.Username,
		"loginType": "normal",
	}
	bodyjson, _ := json.Marshal(body)
	req := h.Request("POST", fmt.Sprintf("%s%s", h.HostPath, "/login"), string(bodyjson), nil)
	resp, err := h.Client.Do(&req)
	if err != nil {
		clog.Error("login fail, %v", err)
		return err
	}
	user.Cookies = resp.Cookies()
	user.AuthType = 0
	return nil
}

// get token
func (h *HttpHelper) AuthToken() *HttpHelper {
	h.GetToken(&h.Admin)
	h.GetToken(&h.TenantAdmin)
	h.GetToken(&h.ProjectAdmin)
	h.GetToken(&h.User)
	return h
}

// get token by ak sk
func (h *HttpHelper) GetToken(user *AuthUser) error {
	req := h.Request("GET", fmt.Sprintf("%s/api/v1/cube/key/token?accessKey=%s&secretKey=%s", h.HostPath, user.Ak, user.Sk), "", nil)
	resp, err := h.Client.Do(&req)
	if err != nil {
		clog.Error("get token by ak sk fail, %v", err)
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		clog.Error("get token by ak sk fail, %v", err)
		return err
	}
	var ret map[string]interface{}
	err = json.Unmarshal(body, &ret)
	if err != nil {
		clog.Error("parse token json fail, %v", err)
		return err
	}
	if v, ok := ret["token"]; ok {
		user.Token = v.(string)
		user.AuthType = 1
		return nil
	}
	clog.Error("token not exist, ak=%s", user.Ak)
	return fmt.Errorf("token not exist, ak=%s", user.Ak)
}

// format url
func (h *HttpHelper) FormatUrl(url string) string {
	return fmt.Sprintf("%s%s%s", h.HostPath, "/api/v1/cube", url)
}

// get
func (h *HttpHelper) Get(urlVal string, header map[string]string) http.Request {
	return h.RequestByUser(http.MethodGet, urlVal, "", header, "admin")
}

// post
func (h *HttpHelper) Post(urlVal, body string, header map[string]string) http.Request {
	return h.RequestByUser(http.MethodPost, urlVal, body, header, "admin")
}

// delete
func (h *HttpHelper) Delete(urlVal string) http.Request {
	return h.RequestByUser(http.MethodDelete, urlVal, "", nil, "admin")
}

// put
func (h *HttpHelper) Put(urlVal, body string, header map[string]string) http.Request {
	return h.RequestByUser(http.MethodPut, urlVal, body, header, "admin")
}

// default request by admin
func (h *HttpHelper) Request(method, urlVal, data string, header map[string]string) http.Request {
	return h.RequestByUser(method, urlVal, data, header, "admin")
}

// request by admin\tenantAdmin\projectAdmin\user
func (h *HttpHelper) RequestByUser(method, urlVal, data string, header map[string]string, user string) http.Request {
	var req *http.Request

	urlArr := strings.Split(urlVal, "?")
	if len(urlArr) == 2 {
		urlVal = urlArr[0] + "?" + url.PathEscape(urlArr[1])
	}
	if data == "" {
		req, _ = http.NewRequest(method, urlVal, nil)
	} else {
		req, _ = http.NewRequest(method, urlVal, strings.NewReader(data))
	}
	switch user {
	case "admin":
		if h.Admin.AuthType == 0 {
			for _, c := range h.Admin.Cookies {
				req.AddCookie(c)
			}
		} else {
			req.Header.Add(constants.AuthorizationHeader, "bearer "+h.Admin.Token)
		}
	case "tenantAdmin":
		if h.TenantAdmin.AuthType == 0 {
			for _, c := range h.Admin.Cookies {
				req.AddCookie(c)
			}
		} else {
			req.Header.Add(constants.AuthorizationHeader, "bearer "+h.TenantAdmin.Token)
		}
	case "projectAdmin":
		if h.ProjectAdmin.AuthType == 0 {
			for _, c := range h.ProjectAdmin.Cookies {
				req.AddCookie(c)
			}
		} else {
			req.Header.Add(constants.AuthorizationHeader, "bearer "+h.ProjectAdmin.Token)
		}
	case "user":
		if h.User.AuthType == 0 {
			for _, c := range h.User.Cookies {
				req.AddCookie(c)
			}
		} else {
			req.Header.Add(constants.AuthorizationHeader, "bearer "+h.User.Token)
		}

	}
	req.Header.Add("Content-Type", "application/json")
	for k, v := range header {
		req.Header.Add(k, v)
	}

	return *req
}

type MultiRequestResponse struct {
	Resp *http.Response
	Err  error
}

// multi user request test
func (h *HttpHelper) MultiUserRequest(method, url, body string, header map[string]string) map[string]MultiRequestResponse {
	ret := make(map[string]MultiRequestResponse)

	r1 := h.RequestByUser(method, h.FormatUrl(url), body, header, "admin")
	resp1, err1 := h.Client.Do(&r1)
	ret["admin"] = MultiRequestResponse{resp1, err1}

	r2 := h.RequestByUser(method, h.FormatUrl(url), body, header, "tenantAdmin")
	resp2, err2 := h.Client.Do(&r2)
	ret["tenantAdmin"] = MultiRequestResponse{resp2, err2}

	r3 := h.RequestByUser(method, h.FormatUrl(url), body, header, "projectAdmin")
	resp3, err3 := h.Client.Do(&r3)
	ret["projectAdmin"] = MultiRequestResponse{resp3, err3}

	r4 := h.RequestByUser(method, h.FormatUrl(url), body, header, "user")
	resp4, err4 := h.Client.Do(&r4)
	ret["user"] = MultiRequestResponse{resp4, err4}

	return ret
}
