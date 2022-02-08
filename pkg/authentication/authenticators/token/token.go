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
	"fmt"
	"net/http"
	"strings"

	"k8s.io/api/authentication/v1beta1"

	"github.com/kubecube-io/kubecube/pkg/authentication/authenticators/jwt"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
)

func GetTokenFromReq(req *http.Request) (string, error) {
	// get token from header
	var bearerToken = req.Header.Get(constants.AuthorizationHeader)
	if bearerToken == "" {
		clog.Debug("get bearer token from header is empty")
		// get token from cookie
		cookie, err := req.Cookie(constants.AuthorizationHeader)
		if err != nil {
			return "", err
		}
		bearerToken = cookie.Value
		if bearerToken == "" {
			clog.Warn("get bearer token from cookie is empty")
			return "", nil
		}
	}

	// parse bearer token
	var parts []string
	if strings.Contains(bearerToken, " ") {
		parts = strings.Split(bearerToken, " ")
	} else {
		parts = strings.Split(bearerToken, "+")
	}
	if len(parts) < 2 || strings.ToLower(parts[0]) != strings.ToLower(jwt.BearerTokenPrefix) {
		return "", fmt.Errorf("bearer token: %s format is wrong", bearerToken)
	}
	return parts[1], nil
}

func GetUserFromReq(req *http.Request) (*v1beta1.UserInfo, error) {
	token, err := GetTokenFromReq(req)
	if err != nil {
		return nil, err
	}

	userInfo, err := jwt.GetAuthJwtImpl().Authentication(token)
	if err != nil {
		return nil, err
	}

	return userInfo, nil
}
