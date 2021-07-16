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
	"github.com/gin-gonic/gin"
	"github.com/kubecube-io/kubecube/pkg/authenticator/jwt"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"strings"
)

func GetTokenFromReq(c *gin.Context) string {
	// get token from header
	var bearerToken = c.Request.Header.Get(constants.AuthorizationHeader)
	if bearerToken == "" {
		clog.Debug("get bearer token from header is empty")
		// get token from cookie
		bearerToken, _ = c.Cookie(constants.AuthorizationHeader)
		if bearerToken == "" {
			clog.Warn("get bearer token from cookie is empty")
			return ""
		}
	}

	// parse bearer token
	parts := strings.Split(bearerToken, " ")
	if len(parts) < 2 || strings.ToLower(parts[0]) != strings.ToLower(jwt.BearerTokenPrefix) {
		return ""
	}
	return parts[1]
}

func GetUserFromReq(c *gin.Context) string {
	token := GetTokenFromReq(c)
	if token != "" {
		claims, err := jwt.ParseToken(token)
		if err == nil {
			return claims.UserInfo.Username
		}
	}
	return ""
}
