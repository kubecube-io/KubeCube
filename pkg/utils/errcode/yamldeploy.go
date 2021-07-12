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

import "net/http"

var (
	MissNamespaceInObj = New(&ErrorInfo{http.StatusBadRequest, "miss namespace in .metadata.namespace."})
	MissNameInObj      = New(&ErrorInfo{http.StatusBadRequest, "miss namespace in .metadata.name."})
	deployYamlFail     = New(&ErrorInfo{http.StatusBadRequest, "deploy by yaml fail, %v"})
	createMappingFail  = New(&ErrorInfo{http.StatusBadRequest, "create mapping fail, %v"})
)

func CreateMappingError(err string) *ErrorInfo {
	return New(createMappingFail, err)
}

func DeployYamlError(err string) *ErrorInfo {
	return New(deployYamlFail, err)
}
