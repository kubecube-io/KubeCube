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

package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/kubecube-io/kubecube/pkg/authentication/identityprovider"
	"github.com/kubecube-io/kubecube/pkg/clog"
)

const githubUserUrl = "https://api.github.com/user"

type githubProvider struct {
	ClientID       string `json:"clientID" yaml:"clientID"`
	ClientSecret   string `json:"clientSecret" yaml:"clientSecret"`
	GitHubIsEnable bool   `json:"gitHubIsEnable" yaml:"gitHubIsEnable"`
}

type tokenInfo struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

type githubIdentity struct {
	Login             string    `json:"login"`
	ID                int       `json:"id"`
	NodeID            string    `json:"node_id"`
	AvatarURL         string    `json:"avatar_url"`
	GravatarID        string    `json:"gravatar_id"`
	URL               string    `json:"url"`
	HTMLURL           string    `json:"html_url"`
	FollowersURL      string    `json:"followers_url"`
	FollowingURL      string    `json:"following_url"`
	GistsURL          string    `json:"gists_url"`
	StarredURL        string    `json:"starred_url"`
	SubscriptionsURL  string    `json:"subscriptions_url"`
	OrganizationsURL  string    `json:"organizations_url"`
	ReposURL          string    `json:"repos_url"`
	EventsURL         string    `json:"events_url"`
	ReceivedEventsURL string    `json:"received_events_url"`
	Type              string    `json:"type"`
	SiteAdmin         bool      `json:"site_admin"`
	Name              string    `json:"name"`
	Company           string    `json:"company"`
	Blog              string    `json:"blog"`
	Location          string    `json:"location"`
	Email             string    `json:"email"`
	Hireable          bool      `json:"hireable"`
	Bio               string    `json:"bio"`
	PublicRepos       int       `json:"public_repos"`
	PublicGists       int       `json:"public_gists"`
	Followers         int       `json:"followers"`
	Following         int       `json:"following"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	TwitterUsername   string    `json:"twitter_username"`
}

func (g githubIdentity) GetRespHeader() http.Header {
	return nil
}

func (g githubIdentity) GetUserName() string {
	return g.Login
}

func (g githubIdentity) GetGroup() string {
	return ""
}

func (g githubIdentity) GetUserEmail() string {
	return g.Email
}

func GetProvider() githubProvider {
	config := getConfig()
	return githubProvider{
		ClientID:       config.ClientID,
		ClientSecret:   config.ClientSecret,
		GitHubIsEnable: config.GitHubIsEnable,
	}
}

func (g *githubProvider) IdentityExchange(code string) (identityprovider.Identity, error) {
	if g.ClientID == "" || g.ClientSecret == "" {
		clog.Error("clientId or clientSecret is null")
		return nil, errors.New("clientId or clientSecret is null")
	}

	// get token
	url := fmt.Sprintf("https://github.com/login/oauth/access_token?client_id=%s&client_secret=%s&code=%s", g.ClientID, g.ClientSecret, code)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		clog.Error("new http post request err: %v", err)
		return nil, err
	}
	req.Header.Set("accept", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		clog.Error("request to github for token error: %v", err)
		return nil, err
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		clog.Error("read response error: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if strings.Contains(string(respBody), "bad_verification_code") {
		clog.Error("%v", string(respBody))
		return nil, errors.New("bad verification code")
	}

	var t tokenInfo
	err = json.Unmarshal(respBody, &t)
	if err != nil {
		return nil, err
	}

	// get user info by token
	req, err = http.NewRequest(http.MethodGet, githubUserUrl, nil)
	if err != nil {
		clog.Error("new http get request err: %v", err)
		return nil, err
	}
	req.Header.Set("accept", "application/json")
	req.Header.Set("Authorization", "token "+t.AccessToken)
	resp, err = client.Do(req)
	if err != nil {
		clog.Error("request to github for user info error: %v", err)
		return nil, err
	}

	respBody, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		clog.Error("read response error: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		clog.Error("response code is not 200")
		return nil, errors.New("response code is not 200")
	}

	var identity githubIdentity
	err = json.Unmarshal(respBody, &identity)
	if err != nil {
		return nil, err
	}

	return identity, nil
}
