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
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/kubecube-io/kubecube/pkg/authentication"
	"github.com/kubecube-io/kubecube/pkg/authentication/identityprovider"
	"github.com/kubecube-io/kubecube/pkg/clog"
)

var Config = authentication.GenericConfig{}

type HeaderProvider struct {
	URL                string
	Method             string
	Scheme             string
	InsecureSkipVerify bool
	TLSCert            string
	TLSKey             string
}

type GenericIdentity struct {
	Username string
	Header   http.Header
}

func (g *GenericIdentity) GetRespHeader() http.Header {
	return g.Header
}

// GetUserEmail generic auth method response does not include email
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
	return HeaderProvider{Config.URL, Config.Method, Config.Scheme,
		Config.InsecureSkipVerify, Config.TLSCert, Config.TLSKey}
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
		return nil, fmt.Errorf("new http request (%v/%v) err: %v", h.URL, h.Method, err)
	}
	req.Header = headers
	tr := &http.Transport{}
	if h.Scheme == "https" {
		cfg := &tls.Config{}
		if h.InsecureSkipVerify == true {
			cfg = &tls.Config{
				InsecureSkipVerify: true,
			}
		} else {
			if h.TLSCert == "" || h.TLSKey == "" {
				return nil, fmt.Errorf("generic auth cert is %s, key is %s", h.TLSCert, h.TLSKey)
			}
			certBytes := []byte(h.TLSCert)
			ketBytes := []byte(h.TLSKey)
			c, err := tls.X509KeyPair(certBytes, ketBytes)
			if err != nil {
				clog.Error("%v", err)
				return nil, err
			}
			cfg = &tls.Config{
				Certificates: []tls.Certificate{c},
			}
		}
		tr = &http.Transport{
			TLSClientConfig: cfg,
		}
	}
	client := &http.Client{Timeout: 30 * time.Second, Transport: tr}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to generic auth error: %v", err)
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response error: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("response from the third party is not ok, response is %s", string(respBody))
	}

	var respMap map[string]interface{}
	if err = json.Unmarshal(respBody, &respMap); err != nil {
		return nil, fmt.Errorf("json unmarshal error: %v", err)
	}

	name := ""
	if username := respMap["name"]; username != nil {
		n, ok := username.(string)
		if !ok {
			return nil, errors.New("username is not string type")
		}
		name = n
	}
	respHeader := resp.Header

	return &GenericIdentity{
		Username: name,
		Header:   respHeader,
	}, nil
}
