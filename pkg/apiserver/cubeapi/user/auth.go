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
	"encoding/hex"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"k8s.io/api/authentication/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/kubecube-io/kubecube/pkg/apis/user/v1"
	"github.com/kubecube-io/kubecube/pkg/authentication/authenticators/jwt"
	"github.com/kubecube-io/kubecube/pkg/authentication/identityprovider/github"
	"github.com/kubecube-io/kubecube/pkg/authentication/identityprovider/ldap"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"github.com/kubecube-io/kubecube/pkg/utils/md5util"
	"github.com/kubecube-io/kubecube/pkg/utils/response"
)

type LoginInfo struct {
	Name      string       `json:"name"`
	Password  string       `json:"password"`
	LoginType v1.LoginType `json:"loginType"`
}

const (
	ldapUserNamePrefix   = "ldap-"
	gitHubUserNamePrefix = "github-"
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
		ldapUser, errInfo := ldapLogin(c, name, password)
		if errInfo != nil {
			response.FailReturn(c, errInfo)
			return
		}
		user = ldapUser
	} else if loginType == v1.NormalLogin {
		normalUser, errInfo := normalLogin(c, name, password)
		if errInfo != nil {
			response.FailReturn(c, errInfo)
			return
		}
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
	authJwtImpl := jwt.GetAuthJwtImpl()
	token, err := authJwtImpl.GenerateToken(&v1beta1.UserInfo{Username: name})
	if err != nil {
		clog.Warn(err.Error())
		response.FailReturn(c, errcode.AuthenticateError)
		return
	}
	bearerToken := jwt.BearerTokenPrefix + " " + token
	c.SetCookie(constants.AuthorizationHeader, bearerToken, int(authJwtImpl.TokenExpireDuration), "/", "", false, true)

	user.Spec.Password = ""
	response.SuccessReturn(c, user)
	return
}

func GitHubLogin(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		clog.Error("code is null")
		response.FailReturn(c, errcode.AuthenticateError)
		return
	}

	provider := github.GetProvider()
	if !provider.GitHubIsEnable {
		clog.Error("github auth is disabled")
		response.FailReturn(c, errcode.AuthenticateError)
		return
	}

	userInfo, err := provider.IdentityExchange(code)
	if err != nil {
		response.FailReturn(c, errcode.AuthenticateError)
		return
	}
	clog.Info("user %s auth success by github", userInfo.GetUserName())

	// get user by name
	userName := hex.EncodeToString([]byte(userInfo.GetUserName()))
	userFind, respInfo := GetUserByName(c, gitHubUserNamePrefix+userName)
	if respInfo != nil {
		response.FailReturn(c, respInfo)
		return
	}
	if userFind != nil && userFind.Spec.State == v1.ForbiddenState {
		response.FailReturn(c, errcode.UserIsDisabled)
		return
	}

	// if user first login, create user
	user := &v1.User{}
	if userFind == nil {
		user.Name = gitHubUserNamePrefix + userName
		user.Spec.DisplayName = gitHubUserNamePrefix + userInfo.GetUserName()
		user.Spec.LoginType = v1.GitHubLogin
		user.Spec.Password = md5util.GetMD5Salt(uuid.New().String())
		user.Labels = make(map[string]string)
		user.Labels["name"] = userInfo.GetUserName()
		if respInfo = CreateUserImpl(c, user); respInfo != nil {
			response.FailReturn(c, respInfo)
			return
		}
	} else {
		user = userFind
	}

	// update user login information
	user.Status.LastLoginIP = c.ClientIP()
	user.Status.LastLoginTime = &metav1.Time{Time: time.Now()}
	respInfo = UpdateUserStatusImpl(c, user)
	if respInfo != nil {
		response.FailReturn(c, respInfo)
		return
	}

	// generate token and return
	authJwtImpl := jwt.GetAuthJwtImpl()
	token, errInfo := authJwtImpl.GenerateToken(&v1beta1.UserInfo{Username: userName})
	bearerToken := jwt.BearerTokenPrefix + " " + token
	if errInfo != nil {
		response.FailReturn(c, errcode.AuthenticateError)
		return
	}
	c.SetCookie(constants.AuthorizationHeader, bearerToken, int(authJwtImpl.TokenExpireDuration), "/", "", false, true)
	c.Set(constants.EventAccountId, user.Name)

	response.SuccessReturn(c, user)
	return
}

func normalLogin(c *gin.Context, name string, password string) (*v1.User, *errcode.ErrorInfo) {
	// name and password login
	user, respInfo := GetUserByName(c, name)
	if respInfo != nil {
		return nil, respInfo
	}
	if user == nil {
		return nil, errcode.AuthenticateError
	}
	if user.Spec.Password != md5util.GetMD5Salt(password) {
		return nil, errcode.AuthenticateError
	}
	if user.Spec.State == v1.ForbiddenState {
		return nil, errcode.UserIsDisabled
	}
	clog.Info("user %s login success with password", name)
	return user, nil
}

func ldapLogin(c *gin.Context, name string, password string) (*v1.User, *errcode.ErrorInfo) {
	// get user by name
	baseName := hex.EncodeToString([]byte(name))
	user, respInfo := GetUserByName(c, ldapUserNamePrefix+baseName)
	if respInfo != nil {
		return nil, respInfo
	}
	if user != nil && user.Spec.State == v1.ForbiddenState {
		return nil, errcode.UserIsDisabled
	}

	// ldap login
	ldapProvider := ldap.GetProvider()
	_, err := ldapProvider.Authenticate(name, password)
	if err != nil {
		return nil, errcode.AuthenticateError
	}
	clog.Info("user %s auth success by ldap", name)

	// if user first login, create user
	if user == nil {
		user = &v1.User{}
		user.Name = ldapUserNamePrefix + baseName
		user.Spec.LoginType = v1.LDAPLogin
		user.Spec.Password = md5util.GetMD5Salt(uuid.New().String())
		user.Labels = make(map[string]string)
		user.Labels["name"] = name
		if respInfo = CreateUserImpl(c, user); respInfo != nil {
			return nil, respInfo
		}
	}
	return user, nil
}
