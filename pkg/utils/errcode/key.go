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
	UserNotExistErr   = New(&ErrorInfo{Code: http.StatusBadRequest, Message: "user not exist."})
	KeyNotExistErr    = New(&ErrorInfo{Code: http.StatusBadRequest, Message: "key not exist."})
	NotMatchErr       = New(&ErrorInfo{Code: http.StatusBadRequest, Message: "key and user not match."})
	SecretNotMatchErr = New(&ErrorInfo{Code: http.StatusBadRequest, Message: "secretkey and accesskey not match."})
	MaxKeyErr         = New(&ErrorInfo{Code: http.StatusBadRequest, Message: "already have 5 credentials, can't create more."})
	ServerErr         = New(&ErrorInfo{Code: http.StatusInternalServerError, Message: "server error."})
)
