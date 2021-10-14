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

package auth

import (
	"net/url"

	"github.com/gin-gonic/gin"

	"github.com/kubecube-io/kubecube/pkg/authentication/authenticators/jwt"
	"github.com/kubecube-io/kubecube/pkg/authentication/authenticators/token"
	"github.com/kubecube-io/kubecube/pkg/authentication/identityprovider/generic"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"github.com/kubecube-io/kubecube/pkg/utils/response"
)

const (
	post  = "POST"
	get   = "GET"
	del   = "DELETE"
	put   = "PUT"
	patch = "PATCH"
)

var whiteList = map[string]string{
	constants.ApiPathRoot + "/login":                       post,
	constants.ApiPathRoot + "/audit":                       post,
	constants.ApiPathRoot + "/key/token":                   get,
	constants.ApiPathRoot + "/authorization/access":        post,
	constants.ApiPathRoot + "/oauth/redirect":              get,
	constants.ApiPathRoot + "/extend/configmap/:configmap": get,
}

func withinWhiteList(url *url.URL, method string, whiteList map[string]string) bool {
	queryUrl := url.Path
	if _, ok := whiteList[queryUrl]; ok {
		if whiteList[queryUrl] == method {
			return true
		}
		return false
	}
	return false
}

func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !withinWhiteList(c.Request.URL, c.Request.Method, whiteList) {
			if generic.Config.GenericAuthIsEnable {
				h := generic.GetProvider()
				user, err := h.Authenticate(c.Request.Header)
				if err != nil || user == nil {
					clog.Error("generic auth error: %v", err)
					response.FailReturn(c, errcode.AuthenticateError)
					return
				}
				newToken, errInfo := jwt.GenerateToken(user.GetUserName(), 0)
				if errInfo != nil {
					response.FailReturn(c, errInfo)
					return
				}
				b := jwt.BearerTokenPrefix + " " + newToken
				c.Request.Header.Set(constants.AuthorizationHeader, b)
				for k, v := range user.GetRespHeader() {
					if k == "Cookie" {
						if len(v) > 1 {
							c.Header(k, v[0])
						}
						break
					}
				}
			} else {
				userToken := token.GetTokenFromReq(c)
				if userToken == "" {
					response.FailReturn(c, errcode.AuthenticateError)
					return
				}

				newToken, respInfo := jwt.RefreshToken(userToken)
				if respInfo != nil {
					clog.Error("refresh token failed")
					response.FailReturn(c, respInfo)
					return
				}

				v := jwt.BearerTokenPrefix + " " + newToken

				c.Request.Header.Set(constants.AuthorizationHeader, v)
				c.SetCookie(constants.AuthorizationHeader, v, int(jwt.Config.TokenExpireDuration), "/", "", false, true)
			}
			c.Next()
		}
	}
}
