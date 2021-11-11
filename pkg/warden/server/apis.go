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

package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"k8s.io/api/authentication/v1beta1"

	"github.com/kubecube-io/kubecube/pkg/authentication/authenticators/jwt"
	"github.com/kubecube-io/kubecube/pkg/clog"
)

const (
	authAPIVersion = "authentication.k8s.io/v1beta1"
	authKind       = "TokenReview"
)

func healthCheck(c *gin.Context) {
	c.String(http.StatusOK, "healthy")
}

func authenticate(c *gin.Context) {
	var authResp = v1beta1.TokenReview{}

	// check struct
	var authReq = v1beta1.TokenReview{}
	if err := c.ShouldBindJSON(&authReq); err != nil {
		authResp.Status.Authenticated = false
		clog.Error("parse auth request body error: %s", err)
		c.JSON(http.StatusBadRequest, authResp)
		c.Abort()
		return
	}
	clog.Debug("auth request: %+v", authResp)

	// check param
	if authReq.APIVersion != authAPIVersion || authReq.Kind != authKind {
		authResp.Status.Authenticated = false
		clog.Error("auth apiVersion or kind is invalid.")
		c.JSON(http.StatusBadRequest, authResp)
		c.Abort()
		return
	}

	// parse jwt
	authJwtImpl := jwt.GetAuthJwtImpl()
	userInfo, err := authJwtImpl.Authentication(authReq.Spec.Token)
	if err != nil {
		authResp.Status.Authenticated = false
		clog.Error("jwt token invalid.")
		c.JSON(http.StatusOK, authResp)
		c.Abort()
		return
	}

	// set response
	authResp.Status.Authenticated = true
	authResp.Status.User = *userInfo

	clog.Debug("auth success, response: %+v", authResp)

	c.JSON(http.StatusOK, authResp)
}

type AuthSpec struct {
	Token string `json:"token"`
}

type AuthStatus struct {
	Authenticated bool        `json:"authenticated"`
	User          *jwt.Claims `json:"user,omitempty"`
}
