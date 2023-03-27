/*
Copyright 2022 KubeCube Authors

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

package request

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"github.com/kubecube-io/kubecube/pkg/utils/constants"
)

func AddFieldManager(req *http.Request, username string) error {
	for _, r := range username {
		if !unicode.IsPrint(r) {
			return fmt.Errorf("username not printable")
		}
	}

	username = "cube-" + username

	if len(username) > 128 {
		return fmt.Errorf("username should not be longer than 128")
	}

	return AddQuery(req, constants.FieldManager, username)
}

func AddQuery(req *http.Request, key, value string) error {
	query, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		return err
	}
	query.Set(key, value)
	newQueryString := query.Encode()
	req.URL.RawQuery = newQueryString
	path := strings.Split(req.RequestURI, "?")[0]
	req.RequestURI = path + "?" + newQueryString

	return nil
}
