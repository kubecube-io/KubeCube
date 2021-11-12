package authproxy

import (
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/kubecube-io/kubecube/pkg/authentication/authenticators"
	"github.com/kubecube-io/kubecube/pkg/authentication/authenticators/jwt"
	"github.com/kubecube-io/kubecube/pkg/utils/ctls"
	"k8s.io/api/authentication/v1beta1"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
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
	h.authMgr = &jwt.AuthJwtImpl{}

	// get cluster info from rest config
	cfg, err := ctrl.GetConfig()
	if err != nil {
		return nil, err
	}

	target, err := url.Parse(cfg.Host)
	if err != nil {
		return nil, err
	}

	// k8s-apiserver needs extract user info from client cert
	ts, err := ctls.MakeMTlsTransportByPem(cfg.CAData, cfg.CertData, cfg.KeyData)
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
	//_, err := token.GetUserFromReq(r)
	//if err != nil {
	//}

	var user v1beta1.UserInfo

	// impersonate given user to access k8s-apiserver
	r.Header.Set(impersonateUserKey, user.Username)

	h.proxy.ServeHTTP(w, r)
}
