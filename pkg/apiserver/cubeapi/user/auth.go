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

package user

import (
	"github.com/kubecube-io/kubecube/pkg/authentication/authenticators/jwt"
	"github.com/kubecube-io/kubecube/pkg/authentication/identityprovider/ldap"
	"time"

	"github.com/kubecube-io/kubecube/pkg/utils/constants"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	v1 "github.com/kubecube-io/kubecube/pkg/apis/user/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"github.com/kubecube-io/kubecube/pkg/utils/md5util"
	"github.com/kubecube-io/kubecube/pkg/utils/response"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type LoginInfo struct {
	Name      string       `json:"name"`
	Password  string       `json:"password"`
	LoginType v1.LoginType `json:"loginType"`
}

const (
	ldapUserNamePrefix = "ldap-"
)

// @Summary login
// @Description user login by password or ldap
// @Tags user
// @Accept  json
// @Produce  json
// @Param  loginInfo body  LoginInfo  true  "user login information"
// @Success 200 {object} response.SuccessInfo
// @Failure 500 {object} errcode.ErrorInfo
// @Router /api/v1/cube/login  [post]
func Login(c *gin.Context) {
	c.Set(constants.EventName, "login")

	// check struct
	var userLoginInfo = LoginInfo{}
	if err := c.ShouldBindJSON(&userLoginInfo); err != nil {
		clog.Error("parse user login body error: %s", err)
		response.FailReturn(c, errcode.InvalidBodyFormat)
		return
	}

	// check params
	name := userLoginInfo.Name
	password := userLoginInfo.Password
	loginType := userLoginInfo.LoginType
	clog.Info("user %s try to login, login type is %s", name, loginType)
	if name == "" || password == "" || loginType == "" {
		response.FailReturn(c, errcode.MissingParamNameOrPwdOrLoginType)
		return
	}

	// login
	user := &v1.User{}
	if loginType == v1.LDAPLogin && ldap.IsLdapOpen() {
		// get user by name
		userFind, respInfo := GetUserByName(c, ldapUserNamePrefix+name)
		if respInfo != nil {
			response.FailReturn(c, respInfo)
			return
		}
		if userFind != nil && userFind.Spec.State == v1.ForbiddenState {
			response.FailReturn(c, errcode.UserIsDisabled)
			return
		}

		// ldap login
		ldapProvider := ldap.GetProvider()
		userInfo, err := ldapProvider.Authenticate(name, password)
		if err != nil {
			response.FailReturn(c, errcode.AuthenticateError)
			return
		}
		clog.Info("user %s auth success by ldap", userInfo.GetUserName())

		// if user first login, create user
		if userFind == nil {
			user.Name = ldapUserNamePrefix + name
			user.Spec.LoginType = v1.LDAPLogin
			user.Spec.Password = md5util.GetMD5Salt(uuid.New().String())
			if respInfo = CreateUserImpl(c, user); respInfo != nil {
				response.FailReturn(c, respInfo)
				return
			}
		} else {
			user = userFind
		}
	} else if loginType == v1.NormalLogin {
		// name and password login
		normalUser, respInfo := GetUserByName(c, name)
		if respInfo != nil {
			response.FailReturn(c, respInfo)
			return
		}
		if normalUser == nil {
			response.FailReturn(c, errcode.AuthenticateError)
			return
		}
		if normalUser.Spec.Password != md5util.GetMD5Salt(password) {
			response.FailReturn(c, errcode.AuthenticateError)
			return
		}
		if normalUser.Spec.State == v1.ForbiddenState {
			response.FailReturn(c, errcode.UserIsDisabled)
			return
		}
		clog.Info("user %s login success with password", name)
		user = normalUser
	} else {
		response.FailReturn(c, errcode.AuthenticateError)
		return
	}

	c.Set(constants.EventAccountId, user.Name)
	// update user login information
	user.Status.LastLoginIP = c.ClientIP()
	user.Status.LastLoginTime = &metav1.Time{Time: time.Now()}
	respInfo := UpdateUserStatusImpl(c, user)
	if respInfo != nil {
		response.FailReturn(c, respInfo)
		return
	}

	// generate token and return
	token, respInfo := jwt.GenerateToken(name, 0)
	bearerToken := jwt.BearerTokenPrefix + " " + token
	if respInfo != nil {
		response.FailReturn(c, respInfo)
		return
	}
	c.SetCookie(constants.AuthorizationHeader, bearerToken, int(jwt.Config.TokenExpireDuration), "/", "", false, true)

	response.SuccessReturn(c, user)
	return
}
