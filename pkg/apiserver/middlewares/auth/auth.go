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
	"regexp"

	"github.com/gin-gonic/gin"
	"k8s.io/api/authentication/v1beta1"

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

var AuthWhiteList = map[string]string{
	constants.ApiPathRoot + "/login":                post,
	constants.ApiPathRoot + "/audit":                post,
	constants.ApiPathRoot + "/key/token":            get,
	constants.ApiPathRoot + "/authorization/access": post,
	constants.ApiPathRoot + "/oauth/redirect":       get,
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

func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !withinWhiteList(c.Request.URL, c.Request.Method, AuthWhiteList) {
			authJwtImpl := jwt.GetAuthJwtImpl()
			if generic.Config.GenericAuthIsEnable {
				h := generic.GetProvider()
				user, err := h.Authenticate(c.Request.Header)
				if err != nil || user == nil {
					clog.Warn("generic auth error: %v", err)
					response.FailReturn(c, errcode.AuthenticateError)
					return
				}
				newToken, err := authJwtImpl.GenerateToken(&v1beta1.UserInfo{Username: user.GetUserName()})
				if err != nil {
					clog.Warn(err.Error())
					response.FailReturn(c, errcode.AuthenticateError)
					return
				}
				b := jwt.BearerTokenPrefix + " " + newToken
				c.Request.Header.Set(constants.AuthorizationHeader, b)
				c.Request.Header.Set(constants.ImpersonateUserKey, user.GetUserName())
				for k, v := range user.GetRespHeader() {
					if k == "Cookie" {
						if len(v) > 1 {
							c.Header(k, v[0])
						}
						break
					}
				}
				c.Set(constants.EventAccountId, user.GetUserName())
			} else {
				userToken, err := token.GetTokenFromReq(c.Request)
				if err != nil {
					clog.Warn(err.Error())
					response.FailReturn(c, errcode.AuthenticateError)
					return
				}

				user, newToken, err := authJwtImpl.RefreshToken(userToken)
				if err != nil {
					clog.Warn(err.Error())
					response.FailReturn(c, errcode.AuthenticateError)
					return
				}

				v := jwt.BearerTokenPrefix + " " + newToken

				c.Request.Header.Set(constants.AuthorizationHeader, v)
				c.Request.Header.Set(constants.ImpersonateUserKey, user.Username)
				c.SetCookie(constants.AuthorizationHeader, v, int(authJwtImpl.TokenExpireDuration), "/", "", false, true)
				c.Set(constants.EventAccountId, user.Username)
			}
			c.Next()
		}
	}
}
