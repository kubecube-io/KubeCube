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

package cubeproxy

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"k8s.io/client-go/rest"

	"github.com/kubecube-io/kubecube/pkg/clog"
)

const defaultProxyHost = "http://0.0.0.0:8081"

type Config struct {
	RawRestConfig   *rest.Config
	ProxyRestConfig *rest.Config

	Options
}

type Options struct {
	ProxyHost      string
	HookDelegator  HookDelegator
	BeforeReqHook  func(*http.Request)
	BeforeRespHook func(*http.Response) error
	ErrorResponder ErrorResponder
}

var (
	defaultBeforeReqHook  = func(req *http.Request) {}
	defaultBeforeRespHook = func(resp *http.Response) error { return nil }
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

type HookDelegator interface {
	NewProxyHook(w http.ResponseWriter, r *http.Request) ProxyHook
}

type ProxyHook interface {
	BeforeReqHook(req *http.Request)
	BeforeRespHook(resp *http.Response) error
	ErrorResponder
}

type ProxyHandler struct {
	rawRestConfig   *rest.Config
	proxyRestConfig *rest.Config
	beforeReqHook   func(req *http.Request)
	beforeRespHook  func(resp *http.Response) error
	errorResponder  ErrorResponder
	hookDelegator   HookDelegator

	// proxy do real proxy action with any inbound stream
	proxy *UpgradeAwareHandler
}

func NewProxy(conf *Config) (*ProxyHandler, error) {
	h := &ProxyHandler{
		rawRestConfig:   conf.RawRestConfig,
		proxyRestConfig: conf.ProxyRestConfig,
		hookDelegator:   conf.HookDelegator,
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

	p := NewUpgradeAwareHandler(target, ts, false, false, proxyHooks{
		director:       conf.BeforeReqHook,
		modifyResponse: conf.BeforeRespHook,
		responder:      conf.ErrorResponder,
	})
	p.UpgradeTransport = upgradeTransport
	p.UseRequestLocation = true

	h.proxy = p

	return h, nil
}

func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.hookDelegator != nil {
		hook := h.hookDelegator.NewProxyHook(w, r)
		h.beforeReqHook = hook.BeforeReqHook
		h.beforeRespHook = hook.BeforeRespHook
		h.errorResponder = hook
		h.proxy.ModifyResponse = hook.BeforeRespHook
		h.proxy.Responder = hook
	}
	h.beforeReqHook(r)
	h.proxy.ServeHTTP(w, r)
}

func (h *ProxyHandler) Run(stop <-chan struct{}) {
	mux := http.NewServeMux()
	mux.Handle("/", h)

	u, err := url.Parse(h.proxyRestConfig.Host)
	if err != nil {
		clog.Fatal("parse host %v failed: %v", h.proxyRestConfig.Host, err)
	}

	srv := &http.Server{Handler: mux, Addr: u.Host}

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
