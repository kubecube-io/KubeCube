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

package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/warden/reporter"
	"github.com/kubecube-io/kubecube/pkg/warden/server/authproxy"
)

var log clog.CubeLogger

const apiPathRoot = "/api/v1/warden"

type Server struct {
	Server    *http.Server
	JwtSecret string
	BindAddr  string
	Port      int
	TlsCert   string
	TlsKey    string

	ready bool
}

func (s *Server) Initialize() error {
	log = clog.WithName("apiserver")

	authProxyHandler, err := authproxy.NewHandler()
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/", authProxyHandler)

	s.Server = &http.Server{Handler: mux, Addr: fmt.Sprintf("%s:%d", s.BindAddr, s.Port)}

	reporter.RegisterCheckFunc(s.readyzCheck)

	return nil
}

func (s *Server) Run(stop <-chan struct{}) {
	go func() {
		err := s.Server.ListenAndServeTLS(s.TlsCert, s.TlsKey)
		if err != nil {
			log.Fatal("warden server start err: %v", err)
		}
	}()

	log.Info("warden server listen in %s:%d", s.BindAddr, s.Port)

	// mark auth proxy server ready
	s.ready = true

	<-stop

	log.Info("Shutting down warden server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.Server.Shutdown(ctx); err != nil {
		log.Fatal("server forced to shutdown:", err)
	}

	log.Info("server exiting")
}

func (s *Server) readyzCheck() bool {
	return s.ready
}
