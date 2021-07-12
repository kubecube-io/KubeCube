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

import "fmt"

var (
	InternalServerError    = New(internalServerError)
	InvalidBodyFormat      = New(invalidBodyFormat)
	InvalidFileType        = New(invalidFileType)
	InvalidResourceTypeErr = New(invalidResourceType)
	InvalidHttpMethod      = New(invalidHttpMethod)
)

func CustomReturn(code int, format string, params ...interface{}) *ErrorInfo {
	return &ErrorInfo{
		Code:    code,
		Message: fmt.Sprintf(format, params...),
	}
}

func CreateResourceError(resourceType string) *ErrorInfo {
	return New(createResourceError, resourceType)
}
func UpdateResourceError(resourceType string) *ErrorInfo {
	return New(updateResourceError, resourceType)
}
func GetResourceError(resourceType string) *ErrorInfo {
	return New(getResourceError, resourceType)
}

func ClusterNotFoundError(clusterName string) *ErrorInfo {
	return New(clusterNotFound, clusterName)
}

func DealError(err error) *ErrorInfo {
	return New(dealErrorType, err.Error())
}

func BadRequest(err error) *ErrorInfo {
	if err == nil {
		return New(badRequest, "")
	}
	return New(badRequest, err.Error())
}
