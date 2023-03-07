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
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/kubecube-io/kubecube/pkg/authentication"
	"github.com/kubecube-io/kubecube/pkg/authentication/identityprovider"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/ctls"
)

var (
	Config       authentication.GenericConfig
	once         sync.Once
	authProvider *HeaderProvider
)

type HeaderProvider struct {
	URL                string
	Method             string
	Scheme             string
	InsecureSkipVerify bool
	CACert             string
	CAKey              string
	TLSCert            string
	TLSKey             string

	Client *http.Client
}

type GenericIdentity struct {
	Username  string
	Header    http.Header
	AccountId string
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

func (g *GenericIdentity) GetAccountId() string {
	return g.AccountId
}

func GetProvider() *HeaderProvider {
	once.Do(func() {
		authProvider = &HeaderProvider{
			URL:                Config.URL,
			Method:             Config.Method,
			Scheme:             Config.Scheme,
			InsecureSkipVerify: Config.InsecureSkipVerify,
			TLSCert:            Config.TLSCert,
			TLSKey:             Config.TLSKey,
			CACert:             Config.CACert,
			CAKey:              Config.CAKey,
		}

		// use transport without tls by default
		authProvider.Client = &http.Client{Timeout: 30 * time.Second, Transport: ctls.DefaultTransport()}

		if Config.Scheme != "https" {
			return
		}

		// here we should use tls config and use insecure transport by default
		authProvider.Client.Transport = ctls.MakeInsecureTransport()
		switch {
		case Config.InsecureSkipVerify:
		case Config.CACert != "" && Config.TLSCert != "" && Config.TLSKey != "":
			tr, err := ctls.MakeMTlsTransportByFile(Config.CACert, Config.TLSCert, Config.TLSKey)
			if err != nil {
				clog.Warn("make mtls transport failed, use insecure by default: %v", err)
				return
			}
			authProvider.Client.Transport = tr
		default:
			clog.Warn("less mtls config file, caCert: %v, tlsCert: %v, tlsKey: %v, use insecure",
				Config.CACert, Config.TLSCert, Config.TLSKey)
		}
	})

	return authProvider
}

func (g *GenericIdentity) GetUserID() string {
	return g.Username
}

func (g *GenericIdentity) GetUsername() string {
	return g.Username
}

func (h *HeaderProvider) Authenticate(headers map[string][]string) (identityprovider.Identity, error) {
	req, err := http.NewRequest(h.Method, h.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("new http request (%v/%v) err: %v", h.URL, h.Method, err)
	}
	req.Header = headers
	resp, err := h.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to generic auth error: %v", err)
	}

	defer resp.Body.Close()
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

	name, accountId := "", ""
	if username := respMap["name"]; username != nil {
		n, ok := username.(string)
		if !ok {
			return nil, errors.New("username is not string type")
		}
		name = n
	}
	if account := respMap["accountId"]; account != nil {
		n, ok := account.(string)
		if !ok {
			return nil, errors.New("accountId is not string type")
		}
		accountId = n
	}
	respHeader := resp.Header

	return &GenericIdentity{
		Username:  name,
		Header:    respHeader,
		AccountId: accountId,
	}, nil
}
