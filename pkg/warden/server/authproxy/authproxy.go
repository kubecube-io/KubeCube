package authproxy

import (
	"context"
	"fmt"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	v1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
	"github.com/kubecube-io/kubecube/pkg/authentication/authenticators"
	"github.com/kubecube-io/kubecube/pkg/authentication/authenticators/jwt"
	"github.com/kubecube-io/kubecube/pkg/authentication/authenticators/token"
	"github.com/kubecube-io/kubecube/pkg/utils/ctls"
	"github.com/kubecube-io/kubecube/pkg/utils/kubeconfig"
	"github.com/kubecube-io/kubecube/pkg/warden/utils"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
)

const (
	impersonateUserKey  = "Impersonate-User"
	impersonateGroupKey = "Impersonate-Group"
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

	ts.DialContext = (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext
	ts.ForceAttemptHTTP2 = true
	ts.MaxIdleConns = 50
	ts.IdleConnTimeout = 60 * time.Second
	ts.TLSHandshakeTimeout = 10 * time.Second
	ts.ExpectContinueTimeout = 1 * time.Second

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
	r.Header.Set(impersonateUserKey, user.Username)

	h.proxy.ServeHTTP(w, r)
}
