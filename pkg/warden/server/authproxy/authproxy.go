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
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	v1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
	"github.com/kubecube-io/kubecube/pkg/authentication/authenticators"
	"github.com/kubecube-io/kubecube/pkg/authentication/authenticators/jwt"
	"github.com/kubecube-io/kubecube/pkg/authentication/authenticators/token"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/ctls"
	"github.com/kubecube-io/kubecube/pkg/utils/kubeconfig"
	"github.com/kubecube-io/kubecube/pkg/warden/utils"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
)

// Handler forwards all the requests to specified k8s-apiserver
// after pass previous authentication
type Handler struct {
	// authMgr has the way to operator jwt token
	authMgr authenticators.AuthNManager

	// cfg holds current cluster info
	cfg *rest.Config

	// proxy do real proxy action with any inbound stream
	proxy *httputil.ReverseProxy
}

func NewHandler() (*Handler, error) {
	h := &Handler{}
	h.authMgr = jwt.GetAuthJwtImpl()

	// get cluster info from rest config
	cluster := v1.Cluster{}
	err := utils.PivotClient.Get(context.Background(), types.NamespacedName{Name: utils.Cluster}, &cluster)
	if err != nil {
		return nil, err
	}

	restConfig, err := kubeconfig.LoadKubeConfigFromBytes(cluster.Spec.KubeConfig)
	if err != nil {
		return nil, err
	}

	target, err := url.Parse(restConfig.Host)
	if err != nil {
		return nil, err
	}

	// k8s-apiserver needs extract user info from client cert
	// we use admin cert to access k8s-apiserver
	ts, err := ctls.MakeMTlsTransportByPem(restConfig.CAData, restConfig.CertData, restConfig.KeyData)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = ts

	h.proxy = proxy

	return h, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// parse token transfer to user info
	user, err := token.GetUserFromReq(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, "token invalid: %v", err)
		return
	}

	clog.Debug("user(%v) access to %v with verb(%v)", user.Username, r.Method)

	// impersonate given user to access k8s-apiserver
	r.Header.Set(constants.ImpersonateUserKey, user.Username)

	h.proxy.ServeHTTP(w, r)
}
