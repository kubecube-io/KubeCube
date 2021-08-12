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
package generic

import (
	"encoding/json"
	"github.com/kubecube-io/kubecube/pkg/authentication"
	"github.com/kubecube-io/kubecube/pkg/authentication/identityprovider"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"io/ioutil"
	"net/http"
)

var Config = authentication.GenericConfig{}

type HeaderProvider struct {
	URL    string `json:"url,omitempty" yaml:"url"`
	Method string `json:"method,omitempty" yaml:"method"`
}

type GenericIdentity struct {
	Username string
	Header   http.Header
}

func (g *GenericIdentity) GetRespHeader() http.Header {
	return g.Header
}

func (g *GenericIdentity) GetUserEmail() string {
	return ""
}

func (g *GenericIdentity) GetUserName() string {
	return g.Username
}

func (g *GenericIdentity) GetGroup() string {
	return ""
}

func GetProvider() HeaderProvider {
	return HeaderProvider{Config.URL, Config.Method}
}

func (g *GenericIdentity) GetUserID() string {
	return g.Username
}

func (g *GenericIdentity) GetUsername() string {
	return g.Username
}

func (h HeaderProvider) Authenticate(headers map[string][]string) (identityprovider.Identity, error) {

	req, err := http.NewRequest(h.Method, h.URL, nil)
	if err != nil {
		clog.Error("new http request err: %s", err)
		return nil, err
	}
	req.Header = headers
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		clog.Error("request to generic auth error: %s", err)
		return nil, err
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		clog.Error("read response error: %v", err)
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		clog.Error("response from the third party is not ok, response is %s", string(respBody))
		return nil, nil
	}

	var respMap map[string]interface{}
	if err = json.Unmarshal(respBody, &respMap); err != nil {
		clog.Error("json unmarshal error: %v", err)
		return nil, err
	}

	username := respMap["name"]
	respHeader := resp.Header

	return &GenericIdentity{
		Username: username.(string),
		Header:   respHeader,
	}, nil
}
