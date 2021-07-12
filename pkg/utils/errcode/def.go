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

package errcode

import (
	"fmt"
	"net/http"
)

type ErrorInfo struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func New(errorInfo *ErrorInfo, params ...interface{}) *ErrorInfo {
	return &ErrorInfo{
		Code:    errorInfo.Code,
		Message: fmt.Sprintf(errorInfo.Message, params...),
	}
}

var (
	// common
	missingParam        = &ErrorInfo{http.StatusBadRequest, "Param %s is missing."}
	clusterNotFound     = &ErrorInfo{http.StatusBadRequest, "cluster %s not found."}
	invalidResourceType = &ErrorInfo{http.StatusBadRequest, "resource type param error."}
	invalidHttpMethod   = &ErrorInfo{http.StatusBadRequest, "not match http method."}
	badRequest          = &ErrorInfo{http.StatusBadRequest, "bad request. %s"}
	// The first parameter is the parameter name, the second parameter is the parameter value, such as ID 111 already exists
	paramNotUnique      = &ErrorInfo{http.StatusBadRequest, "%s %s exists."}
	invalidParamValue   = &ErrorInfo{http.StatusBadRequest, "Param %s value invalid."}
	internalServerError = &ErrorInfo{http.StatusInternalServerError, "Server is busy, please try again."}
	invalidBodyFormat   = &ErrorInfo{http.StatusBadRequest, "Body format invalid."}
	createResourceError = &ErrorInfo{http.StatusInternalServerError, "Create resource %s failed."}
	updateResourceError = &ErrorInfo{http.StatusInternalServerError, "Update resource %s failed."}
	getResourceError    = &ErrorInfo{http.StatusNotFound, "Get resource %s failed."}
	invalidFileType     = &ErrorInfo{http.StatusBadRequest, "File type invalid."}
	dealErrorType       = &ErrorInfo{http.StatusBadRequest, "deal fail, %v."}

	// auth
	authenticateError = &ErrorInfo{http.StatusUnauthorized, "Authenticate failed."}
	userNotExist      = &ErrorInfo{http.StatusBadRequest, "User not exist."}
	invalidToken      = &ErrorInfo{http.StatusUnauthorized, "Token invalid."}
	forbidden         = &ErrorInfo{http.StatusForbidden, "Forbidden."}
	ldapConnectError  = &ErrorInfo{http.StatusInternalServerError, "Connect to LDAP server failed."}
	passwordWrong     = &ErrorInfo{http.StatusUnauthorized, "Username or password is wrong."}
	userIsDisabled    = &ErrorInfo{http.StatusBadRequest, "User is disabled."}
)
