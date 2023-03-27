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

package authproxy

import (
	"context"

	"net/http"
	"net/url"
	"strings"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/kubecube-io/kubecube/pkg/authentication/authenticators"
	"github.com/kubecube-io/kubecube/pkg/authentication/authenticators/jwt"
	"github.com/kubecube-io/kubecube/pkg/authentication/authenticators/token"
	"github.com/kubecube-io/kubecube/pkg/belongs"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	requestutil "github.com/kubecube-io/kubecube/pkg/utils/request"
	"github.com/kubecube-io/kubecube/pkg/warden/server/authproxy/proxy"
)

// Handler forwards all the requests to specified k8s-apiserver
// after pass previous authentication
type Handler struct {
	// authMgr has the way to operator jwt token
	authMgr authenticators.AuthNManager

	// cfg holds current cluster info
	// cfg *rest.Config

	cli client.Client

	// proxy do real proxy action with any inbound stream
	proxy *proxy.UpgradeAwareHandler
}

func NewHandler(localClusterKubeConfig string) (*Handler, error) {
	h := &Handler{}
	h.authMgr = jwt.GetAuthJwtImpl()

	// get cluster info from rest config
	restConfig, err := clientcmd.BuildConfigFromFlags("", localClusterKubeConfig)
	if err != nil {
		return nil, err
	}

	cli, err := client.NewClientFor(context.Background(), restConfig)
	if err != nil {
		return nil, err
	}

	h.cli = cli

	host := restConfig.Host
	if !strings.HasSuffix(host, "/") {
		host = host + "/"
	}
	target, err := url.Parse(host)
	if err != nil {
		return nil, err
	}

	responder := &responder{}
	ts, err := rest.TransportFor(restConfig)
	if err != nil {
		return nil, err
	}

	upgradeTransport, err := makeUpgradeTransport(restConfig, 30*time.Second)
	if err != nil {
		return nil, err
	}

	p := proxy.NewUpgradeAwareHandler(target, ts, false, false, responder)
	p.UpgradeTransport = upgradeTransport
	p.UseRequestLocation = true

	h.proxy = p

	return h, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// parse token transfer to user info
	userInfo, err := token.GetUserFromReq(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	clog.Debug("user(%v) access to %v with verb(%v)", userInfo.Username, r.URL.Path, r.Method)

	allowed, err := belongs.RelationshipDetermine(context.Background(), h.cli, r.URL.Path, userInfo.Username)
	if err != nil {
		clog.Warn(err.Error())
	} else if !allowed {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	err = requestutil.AddFieldManager(r, userInfo.Username)
	if err != nil {
		clog.Error("fail to add fieldManager due to %s", err)
	}

	// impersonate given user to access k8s-apiserver
	r.Header.Set(constants.ImpersonateUserKey, userInfo.Username)

	h.proxy.ServeHTTP(w, r)
}
