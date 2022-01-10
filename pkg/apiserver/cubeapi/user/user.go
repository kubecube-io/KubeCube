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
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
	"k8s.io/api/authentication/v1beta1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	userv1 "github.com/kubecube-io/kubecube/pkg/apis/user/v1"
	proxy "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/handle"
	"github.com/kubecube-io/kubecube/pkg/authentication/authenticators/jwt"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	"github.com/kubecube-io/kubecube/pkg/utils/audit"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"github.com/kubecube-io/kubecube/pkg/utils/kubeconfig"
	"github.com/kubecube-io/kubecube/pkg/utils/md5util"
	"github.com/kubecube-io/kubecube/pkg/utils/response"
)

const (
	phonePatterns                = `^1([38][0-9]|14[579]|5[^4]|16[6]|7[1-35-8]|9[189])\d{8}$`
	emailPattern                 = `[\w\.]+@\w+\.[a-z]{2,3}(\.[a-z]{2,3})?`
	resourceTypeUser             = "user"
	uploadUserFileParamName      = "userInfoFile"
	downloadUserTemplateFileName = "UserTemplate.csv"
)

type UserList struct {
	Total int        `json:"total"`
	Items []UserItem `json:"items"`
}

type NameValid struct {
	IsValid bool `json:"isValid"`
}

type UserItem struct {
	Name   string            `json:"name,omitempty"`
	Spec   userv1.UserSpec   `json:"spec,omitempty"`
	Status userv1.UserStatus `json:"status,omitempty"`
}

type ResetPwd struct {
	OriginPassword string `json:"originPassword,omitempty"`
	NewPassword    string `json:"newPassword,omitempty"`
	UserName       string `json:"userName,omitempty"`
}

// @Summary create user
// @Description create user manually
// @Tags user
// @Accept  json
// @Produce  json
// @Param user body userv1.User true "user information"
// @Success 200 {object} response.SuccessInfo
// @Failure 500 {object} errcode.ErrorInfo
// @Router /api/v1/cube/user [post]
func CreateUser(c *gin.Context) {
	// check param
	user, errInfo := CheckAndCompleteCreateParam(c)
	if errInfo != nil {
		response.FailReturn(c, errInfo)
		return
	}

	// create user
	if errInfo := CreateUserImpl(c, user); errInfo != nil {
		response.FailReturn(c, errInfo)
		return
	}
	c = audit.SetAuditInfo(c, audit.CreateUser, user.Name)
	response.SuccessReturn(c, nil)
	return
}

func CreateUserImpl(c *gin.Context, user *userv1.User) *errcode.ErrorInfo {

	kClient := clients.Interface().Kubernetes(constants.LocalCluster).Direct()
	err := kClient.Create(c.Request.Context(), user)
	if err != nil {
		clog.Error("create user error: %s", err)
		return errcode.CreateResourceError(resourceTypeUser)
	}

	return nil
}

// @Summary update user
// @Description update user information
// @Tags user
// @Accept  json
// @Produce  json
// @Param user body userv1.User true "user information"
// @Param username path string true "user name"
// @Success 200 {object} response.SuccessInfo
// @Failure 500 {object} errcode.ErrorInfo
// @Router /api/v1/cube/user/:username [put]
func UpdateUser(c *gin.Context) {
	//check struct
	newUser := &userv1.User{}
	if err := c.ShouldBindJSON(newUser); err != nil {
		clog.Error("parse update user body error: %s", err)
		response.FailReturn(c, errcode.InvalidBodyFormat)
		return
	}

	// get origin user
	name := c.Param("username")
	originUser, errInfo := GetUserByName(c, name)
	if errInfo != nil {
		response.FailReturn(c, errInfo)
		return
	}
	if originUser == nil || originUser.Name == "" {
		response.FailReturn(c, errcode.UserNotExist)
		return
	}

	//check param
	user, errInfo := CheckUpdateParam(newUser, originUser)
	if errInfo != nil {
		response.FailReturn(c, errInfo)
		return
	}

	// update user
	errInfo = UpdateUserSpecImpl(c, user)
	if errInfo != nil {
		response.FailReturn(c, errcode.UpdateResourceError(resourceTypeUser))
		return
	}
	c = audit.SetAuditInfo(c, audit.UpdateUser, user.Name)
	response.SuccessReturn(c, nil)
	return
}

func UpdateUserSpecImpl(c *gin.Context, newUser *userv1.User) *errcode.ErrorInfo {
	kClient := clients.Interface().Kubernetes(constants.LocalCluster).Direct()
	err := kClient.Update(c.Request.Context(), newUser)
	if err != nil {
		clog.Error("update user spec to k8s error: %s", err)
		return errcode.UpdateResourceError(resourceTypeUser)
	}
	return nil
}

func UpdateUserStatusImpl(c *gin.Context, newUser *userv1.User) *errcode.ErrorInfo {
	kClient := clients.Interface().Kubernetes(constants.LocalCluster).Direct()
	err := kClient.Status().Update(c.Request.Context(), newUser, &client.UpdateOptions{})
	if err != nil {
		clog.Error("update user status to k8s error: %s", err)
		return errcode.UpdateResourceError(resourceTypeUser)
	}
	return nil
}

// @Summary list user
// @Description fuzzy query user by name or displayName
// @Tags user
// @Param	query	query	string  false  "keyword for query"
// @Param	pageSize	query	int	false "page size"
// @Param	pageNum		query	int	false	"page num"
// @Success 200 {object} UserList
// @Failure 500 {object} errcode.ErrorInfo
// @Router /api/v1/cube/user  [get]
func ListUsers(c *gin.Context) {
	// get all user
	kClient := clients.Interface().Kubernetes(constants.LocalCluster).Cache()
	allUserList := &userv1.UserList{}
	err := kClient.List(c.Request.Context(), allUserList)
	if err != nil {
		clog.Error("list all users from k8s error: %s", err)
		response.FailReturn(c, errcode.GetResourceError(resourceTypeUser))
		return
	}

	// fuzzy query
	query := c.Query("query")
	var filterList = &UserList{}
	for _, user := range allUserList.Items {
		if query == "" || strings.Contains(user.Spec.DisplayName, query) || strings.Contains(user.Name, query) {
			var userResp UserItem
			userResp.Spec = user.Spec
			userResp.Status = user.Status
			userResp.Name = user.Name
			filterList.Items = append(filterList.Items, userResp)
		}
	}

	// page
	filterListJson, err := json.Marshal(filterList)
	if err != nil {
		clog.Error("transform user list struct to json error: %s", err)
		response.FailReturn(c, errcode.InternalServerError)
		return
	}
	pageList := proxy.Filter(c, filterListJson)
	var resultList UserList
	if err = json.Unmarshal(pageList, &resultList); err != nil {
		clog.Error("transform json to user list struct error: %s", err)
		response.FailReturn(c, errcode.InternalServerError)
		return
	}

	response.SuccessReturn(c, resultList)
	return
}

func GetUserByName(c *gin.Context, name string) (*userv1.User, *errcode.ErrorInfo) {
	kClient := clients.Interface().Kubernetes(constants.LocalCluster).Cache()
	user := &userv1.User{}
	var userKey client.ObjectKey
	userKey.Name = name
	err := kClient.Get(c.Request.Context(), userKey, user)

	//if user not found, return nil
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		clog.Error("get user from k8s error: %s", err)
		return nil, errcode.GetResourceError(resourceTypeUser)
	}
	return user, nil
}

func CheckAndCompleteCreateParam(c *gin.Context) (*userv1.User, *errcode.ErrorInfo) {

	// check struct
	user := &userv1.User{}
	if err := c.ShouldBindJSON(&user); err != nil {
		clog.Error("parse create user body error: %s", err)
		return user, errcode.InvalidBodyFormat
	}

	// check param
	// check name only and must
	name := user.Name
	if name == "" {
		return user, errcode.MissingParamUserName
	}
	userFind, errInfo := GetUserByName(c, name)
	if errInfo != nil {
		return user, errInfo
	}
	if userFind != nil {
		return user, errcode.UserNameDuplicated(name)
	}
	user.Name = strings.TrimSpace(name)

	// check password
	password := strings.TrimSpace(user.Spec.Password)
	if password == "" {
		return user, errcode.MissingParamPassword
	}
	matched := checkPwd(password)
	if !matched {
		return user, errcode.InvalidParameterPassword
	}
	user.Spec.Password = md5util.GetMD5Salt(password)

	// if username is empty, set the name to the username
	if user.Spec.DisplayName == "" {
		user.Spec.DisplayName = user.Name
	}

	// check phone
	phone := user.Spec.Phone
	if phone != "" {
		matched, err := regexp.Match(phonePatterns, []byte(strings.TrimSpace(phone)))
		if err != nil {
			clog.Error("regular match user phone error: %s", err)
			return user, errcode.InternalServerError
		} else if !matched {
			return user, errcode.InvalidParameterPhone
		}
		user.Spec.Phone = strings.TrimSpace(phone)
	}

	// check mail
	email := user.Spec.Email
	if email != "" {
		matched, err := regexp.Match(emailPattern, []byte(strings.TrimSpace(email)))
		if err != nil {
			clog.Error("regular match user email error: %s", err)
			return user, errcode.InternalServerError
		} else if !matched {
			return user, errcode.InvalidParameterEmail
		}
		user.Spec.Email = strings.TrimSpace(email)
	}

	// check language
	language := user.Spec.Language
	if language == "" || language != userv1.Chinese && language != userv1.English {
		user.Spec.Language = userv1.Chinese
	}

	// supplement default fields
	user.Spec.LoginType = userv1.NormalLogin
	user.Spec.State = userv1.NormalState
	annotations := make(map[string]string)
	annotations["kubecube.io/sync"] = "true"
	user.Annotations = annotations

	return user, nil
}

func CheckUpdateParam(newUser *userv1.User, originUser *userv1.User) (*userv1.User, *errcode.ErrorInfo) {

	// third-party registered users are only allowed to modify the status
	newState := newUser.Spec.State
	if originUser.Spec.LoginType != userv1.NormalLogin && (newState == userv1.NormalState || newState == userv1.ForbiddenState) {
		originUser.Spec.State = newState
		return originUser, nil
	}

	// check displayName
	if newUser.Spec.DisplayName != "" {
		originUser.Spec.DisplayName = newUser.Spec.DisplayName
	}

	// check password
	newPassword := strings.TrimSpace(newUser.Spec.Password)
	if newPassword != "" {
		if !checkPwd(newPassword) {
			return originUser, errcode.InvalidParameterPassword
		}
		originUser.Spec.Password = md5util.GetMD5Salt(newPassword)
	}

	// check language
	newLanguage := newUser.Spec.Language
	if newLanguage == userv1.Chinese || newLanguage == userv1.English {
		originUser.Spec.Language = newLanguage
	}

	// check email
	newEmail := newUser.Spec.Email
	if newEmail != "" {
		matched, err := regexp.Match(emailPattern, []byte(strings.TrimSpace(newEmail)))
		if err != nil {
			clog.Error("regular match user email error: %s", err)
			return originUser, errcode.InternalServerError
		} else if !matched {
			return originUser, errcode.InvalidParameterEmail
		}
		originUser.Spec.Email = strings.TrimSpace(newEmail)
	}

	// check phone
	newPhone := newUser.Spec.Phone
	if newPhone != "" {
		matched, err := regexp.Match(phonePatterns, []byte(strings.TrimSpace(newPhone)))
		if err != nil {
			clog.Error("regular match user phone error: %s", err)
			return originUser, errcode.InternalServerError
		} else if !matched {
			return originUser, errcode.InvalidParameterPhone
		}
		originUser.Spec.Phone = strings.TrimSpace(newPhone)
	}

	// check status
	if newUser.Spec.State == userv1.NormalState || newUser.Spec.State == userv1.ForbiddenState {
		originUser.Spec.State = newUser.Spec.State
	}

	return originUser, nil
}

// @Summary get import template
// @Description get user information import template
// @Tags user
// @Success 200 {string} string
// @Router /api/v1/cube/user/template  [get]
func DownloadTemplate(c *gin.Context) {
	c.Set(constants.EventName, "download template")
	c.Set(constants.EventResourceType, "file")

	dataBytes := &bytes.Buffer{}
	dataBytes.WriteString("\xEF\xBB\xBF")
	wr := csv.NewWriter(dataBytes)
	data := [][]string{
		{"name", "password", "displayName", "email", "phone"},
		{"ZhangSan", "ZhangSun12345", "ZhangSan", "zhangsan@163.com", "17609873452"},
	}
	if err := wr.WriteAll(data); err != nil {
		clog.Error("write user template file error: %s", err)
		response.FailReturn(c, errcode.InternalServerError)
		return
	}
	// clear
	wr.Flush()
	c.Writer.Header().Set(constants.HttpHeaderContentType, constants.HttpHeaderContentTypeOctet)
	c.Writer.Header().Set(constants.HttpHeaderContentDisposition, fmt.Sprintf("attachment;filename=%s", downloadUserTemplateFileName))
	c.Data(http.StatusOK, "text/csv", dataBytes.Bytes())
	return
}

// @Summary import user
// @Description import and create users from CSV file in batches
// @Tags user
// @Accept multipart/form-data
// @Param userInfoFile formData file true "file"
// @Produce  json
// @Success 200 {object} response.SuccessInfo
// @Failure 500 {object} errcode.ErrorInfo
// @Router /api/v1/cube/user/users [post]
func BatchCreateUser(c *gin.Context) {
	c.Set(constants.EventName, "batch create user")
	c.Set(constants.EventResourceType, "user")

	rFile, err := c.FormFile(uploadUserFileParamName)
	if rFile == nil || err != nil {
		clog.Error("read user file error: %s", err)
		response.FailReturn(c, errcode.MissingParamFile)
		return
	}
	// file name must be end with ".csv"
	if strings.HasSuffix(rFile.Filename, ".csv") != true {
		response.FailReturn(c, errcode.InvalidFileType)
		return
	}

	file, err := rFile.Open()
	if err != nil {
		clog.Error("open file error: %s", err)
		response.FailReturn(c, errcode.InvalidBodyFormat)
		return
	}
	defer file.Close()
	reader := csv.NewReader(bufio.NewReader(file))
	reader.Comma = ';'
	reader.LazyQuotes = true
	userList, err := reader.ReadAll()
	if err != nil {
		clog.Error("parse file to get data error: %s", err)
		response.FailReturn(c, errcode.InvalidBodyFormat)
		return
	}
	var failedMessageList []string
	successCount, failedCount := 0, 0
	for i := 1; i < len(userList); i++ {
		userItem := strings.Split(userList[i][0], ",")
		name, password, displayName, email, phone := userItem[0], userItem[1], userItem[2], userItem[3], userItem[4]
		user := userv1.User{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec:       userv1.UserSpec{Password: password, DisplayName: displayName, Email: email, Phone: phone, LoginType: userv1.NormalLogin},
		}
		if errInfo := CreateUserImpl(c, &user); errInfo != nil {
			failedMessageList = append(failedMessageList, name+": "+errInfo.Message)
			failedCount++
			continue
		}
		successCount++
	}
	var respMap map[string]interface{}
	respMap = make(map[string]interface{})
	respMap["successCount"] = successCount
	respMap["failedCount"] = failedCount
	respMap["failedMessageList"] = failedMessageList

	response.SuccessReturn(c, respMap)
	return
}

// tokenExpiredTime decide when kubeConfig expired at
const tokenExpiredTime = 3600 * 24 * 365 * 10

// GetKubeConfig fetch kubeConfig for specified user
func GetKubeConfig(c *gin.Context) {
	c.Set(constants.EventName, "get kubeconfig")
	c.Set(constants.EventResourceType, "kubeconfig")

	user := c.Query("user")

	authJwtImpl := jwt.GetAuthJwtImpl()
	token, errInfo := authJwtImpl.GenerateTokenWithExpired(&v1beta1.UserInfo{Username: user}, tokenExpiredTime)
	if errInfo != nil {
		response.FailReturn(c, errcode.AuthenticateError)
	}

	clusters := multicluster.Interface().FuzzyCopy()
	cms := make([]*kubeconfig.ConfigMeta, 0, len(clusters))

	for _, cluster := range clusters {
		cm := &kubeconfig.ConfigMeta{
			Cluster: cluster.Name,
			Config:  cluster.Config,
			User:    user,
			Token:   token,
		}
		// set auth proxy server address
		// todo: make port settable
		cm.Config.Host = strings.Replace(cm.Config.Host, "6443", "31443", 1)
		cms = append(cms, cm)
	}

	kubeConfig, err := kubeconfig.BuildKubeConfigForUser(cms)
	if err != nil {
		clog.Error(err.Error())
		response.FailReturn(c, errcode.InternalServerError)
	}

	response.SuccessReturn(c, kubeConfig)
}

// GetMembersByNS show members who in specified namespace
func GetMembersByNS(c *gin.Context) {
	ns := c.Query("namespace")
	if ns == "" {
		response.FailReturn(c, errcode.InvalidBodyFormat)
		return
	}

	ctx := c.Request.Context()
	cli := clients.Interface().Kubernetes(constants.LocalCluster)

	// list all roleBindings in namespace
	roleBindingList := v1.RoleBindingList{}
	err := cli.Cache().List(ctx, &roleBindingList, &client.ListOptions{Namespace: ns})
	if err != nil {
		clog.Error(err.Error())
		response.FailReturn(c, errcode.InternalServerError)
		return
	}

	membersSet := sets.NewString()
	for _, rb := range roleBindingList.Items {
		// match roleBinding witch has specified annotation
		v, ok := rb.Labels[constants.RbacLabel]
		if ok {
			t, err := strconv.ParseBool(v)
			if err != nil {
				clog.Warn("parse member label %v format failed: %v", constants.RbacLabel, err)
				continue
			}
			if t {
				// append user to set
				for _, s := range rb.Subjects {
					if s.Kind == "User" {
						membersSet.Insert(s.Name)
					}
				}
			}
		}
	}

	res := membersSet.List()

	clog.Debug("%v has members: %v", ns, res)

	response.SuccessReturn(c, res)
}

/**
 * the length of password is between 6~18
 * include at least two types of letters, numbers and special symbols
 */
func checkPwd(pwd string) bool {
	var (
		isLetter  = false
		isNumber  = false
		isSpecial = false
	)
	if len(pwd) < 8 || len(pwd) > 20 {
		return false
	}
	for _, s := range pwd {
		switch {
		case unicode.IsLetter(s):
			isLetter = true
		case unicode.IsNumber(s):
			isNumber = true
		case !unicode.IsLetter(s) && !unicode.IsNumber(s):
			isSpecial = true
		}
	}
	if isLetter && isNumber || isLetter && isSpecial || isNumber && isSpecial {
		return true
	}
	return false
}

// @Summary check username
// @Description check username when update user password
// @Tags user
// @Param username path string true "user name"
// @Success 200 {object} NameValid
// @Failure 500 {object} errcode.ErrorInfo
// @Router /api/v1/cube/user/:username [get]
func CheckUserValid(c *gin.Context) {
	name := c.Param("username")
	user, errInfo := GetUserByName(c, name)
	if errInfo != nil {
		response.FailReturn(c, errInfo)
		return
	}
	nameValid := &NameValid{IsValid: true}
	if user == nil || user.Name == "" {
		nameValid.IsValid = false
	}
	response.SuccessReturn(c, nameValid)
	return
}

// @Summary update password
// @Description update user password
// @Tags user
// @Param resetPwd body ResetPwd true "user old and new password"
// @Success 200 {object} response.SuccessInfo
// @Failure 500 {object} errcode.ErrorInfo
// @Router /api/v1/cube/user/pwd [put]
func UpdatePwd(c *gin.Context) {
	//check struct
	resetPwd := &ResetPwd{}
	if err := c.ShouldBindJSON(resetPwd); err != nil {
		clog.Error("parse reset user password body error: %s", err)
		response.FailReturn(c, errcode.InvalidBodyFormat)
		return
	}
	userName := resetPwd.UserName
	oldPwd := resetPwd.OriginPassword
	oldPwdMd5 := md5util.GetMD5Salt(oldPwd)
	newPwd := resetPwd.NewPassword
	user, errInfo := GetUserByName(c, userName)
	if errInfo != nil {
		response.FailReturn(c, errInfo)
		return
	}
	// check original password
	if user.Spec.Password != oldPwdMd5 {
		response.FailReturn(c, errcode.PasswordWrong)
		return
	}
	// check new password
	if !checkPwd(newPwd) {
		response.FailReturn(c, errcode.InvalidParameterPassword)
		return
	}
	// update password
	user.Spec.Password = md5util.GetMD5Salt(newPwd)
	errInfo = UpdateUserSpecImpl(c, user)
	if errInfo != nil {
		response.FailReturn(c, errcode.UpdateResourceError(resourceTypeUser))
		return
	}
	response.SuccessReturn(c, nil)
	return
}
