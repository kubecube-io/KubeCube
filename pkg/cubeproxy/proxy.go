package cubeproxy

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"k8s.io/client-go/rest"

	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/warden/server/authproxy/proxy"
)

const defaultProxyHost = "http://0.0.0.0:8081"

type Config struct {
	RawRestConfig   *rest.Config
	ProxyRestConfig *rest.Config

	Options
}

type Options struct {
	ProxyHost      string
	BeforeReqHook  func(req *http.Request) error
	BeforeRespHook func(resp http.ResponseWriter) error
	ErrorResponder ErrorResponder
}

var (
	defaultBeforeReqHook  = func(req *http.Request) error { return nil }
	defaultBeforeRespHook = func(resp http.ResponseWriter) error { return nil }
)

func NewForConfig(cfg *rest.Config, opts Options) *Config {
	c := &Config{RawRestConfig: cfg}

	if opts.ProxyHost == "" {
		opts.ProxyHost = defaultProxyHost
	}

	if opts.BeforeReqHook == nil {
		opts.BeforeReqHook = defaultBeforeReqHook
	}

	if opts.BeforeRespHook == nil {
		opts.BeforeRespHook = defaultBeforeRespHook
	}

	if opts.ErrorResponder == nil {
		opts.ErrorResponder = &DefaultResponder{}
	}

	c.applyOptions(opts)

	return c
}

func (c *Config) applyOptions(opts Options) {
	proxyCfg := new(rest.Config)
	*proxyCfg = *c.RawRestConfig
	proxyCfg.Host = opts.ProxyHost
	proxyCfg.TLSClientConfig = rest.TLSClientConfig{Insecure: true}
	c.ProxyRestConfig = proxyCfg
	c.Options = opts
}

type ProxyHandler struct {
	rawRestConfig   *rest.Config
	proxyRestConfig *rest.Config
	beforeReqHook   func(req *http.Request) error
	beforeRespHook  func(resp http.ResponseWriter) error
	errorResponder  ErrorResponder

	// proxy do real proxy action with any inbound stream
	proxy *proxy.UpgradeAwareHandler
}

func NewProxy(conf *Config) (*ProxyHandler, error) {
	h := &ProxyHandler{
		rawRestConfig:   conf.RawRestConfig,
		proxyRestConfig: conf.ProxyRestConfig,
		beforeReqHook:   conf.BeforeReqHook,
		beforeRespHook:  conf.BeforeRespHook,
		errorResponder:  conf.ErrorResponder,
	}

	host := conf.RawRestConfig.Host
	if !strings.HasSuffix(host, "/") {
		host = host + "/"
	}
	target, err := url.Parse(host)
	if err != nil {
		return nil, err
	}

	ts, err := rest.TransportFor(conf.RawRestConfig)
	if err != nil {
		return nil, err
	}

	upgradeTransport, err := MakeUpgradeTransport(conf.RawRestConfig, 30*time.Second)
	if err != nil {
		return nil, err
	}

	p := proxy.NewUpgradeAwareHandler(target, ts, false, false, conf.Options.ErrorResponder)
	p.UpgradeTransport = upgradeTransport
	p.UseRequestLocation = true

	h.proxy = p

	return h, nil
}

func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h.beforeReqHook(r); err != nil {
		h.errorResponder(w, r, err)
		return
	}

	h.proxy.ServeHTTP(w, r)

	if err := h.beforeRespHook(w); err != nil {
		h.errorResponder(w, r, err)
		return
	}
}

func (h *ProxyHandler) Run(stop <-chan struct{}) {
	mux := http.NewServeMux()
	mux.Handle("/", h)

	u, err := url.Parse(h.proxyRestConfig.Host)
	if err != nil {
		clog.Fatal("parse host %v failed: %v", h.proxyRestConfig.Host, err)
	}

	srv := &http.Server{Handler: mux, Addr: u.Path}

	go func() {
		clog.Info("proxy server listen in %v", srv.Addr)
		err := srv.ListenAndServe()
		if err != nil {
			clog.Fatal("proxy handler start failed: %v", err)
		}
	}()

	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		clog.Error("proxy handler shut down failed: %v", err)
		return
	}
}
