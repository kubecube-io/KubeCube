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

package warden

import (
	"github.com/kubecube-io/kubecube/pkg/warden/localmgr"
	"github.com/kubecube-io/kubecube/pkg/warden/reporter"
	"github.com/kubecube-io/kubecube/pkg/warden/server"
	"github.com/kubecube-io/kubecube/pkg/warden/syncmgr"
	"github.com/kubecube-io/kubecube/pkg/warden/utils"
)

// Warden has two shapes:
// 1. Running in pivot cluster as a independent process.
// 2. Running in member cluster as a independent process.
type Warden struct {
	*Config

	SyncCtrl  *syncmgr.SyncManager
	Server    *server.Server
	Reporter  *reporter.Reporter
	LocalCtrl *localmgr.LocalManager
}

func NewWardenWithOpts(opts *Config) *Warden {
	w := new(Warden)

	w.Server = &server.Server{
		JwtSecret: opts.JwtSecret,
		BindAddr:  opts.Addr,
		Port:      opts.Port,
		TlsKey:    opts.TlsKey,
		TlsCert:   opts.TlsCert,
	}

	w.Reporter = &reporter.Reporter{
		Cluster:       opts.Cluster,
		PivotCubeHost: opts.PivotCubeHost,
		PeriodSecond:  opts.PeriodSecond,
		WaitSecond:    opts.WaitSecond,
	}

	w.LocalCtrl = &localmgr.LocalManager{
		AllowPrivileged:   opts.AllowPrivileged,
		LeaderElect:       opts.LeaderElect,
		WebhookCert:       opts.WebhookCert,
		WebhookServerPort: opts.WebhookServerPort,
	}

	utils.Cluster = opts.Cluster

	// sync controller only run in member cluster
	if opts.InMemberCluster {
		w.SyncCtrl = &syncmgr.SyncManager{
			PivotClusterKubeConfig: opts.PivotClusterKubeConfig,
		}
	}

	return w
}

func (w Warden) Initialize() error {
	var err error

	if w.SyncCtrl != nil {
		err = w.SyncCtrl.Initialize()
		if err != nil {
			return err
		}
	}

	utils.InitPivotClient()

	err = w.Server.Initialize()
	if err != nil {
		return err
	}

	err = w.LocalCtrl.Initialize()
	if err != nil {
		return err
	}

	err = w.Reporter.Initialize()
	if err != nil {
		return err
	}

	return nil
}

func (w *Warden) Run(stop <-chan struct{}) {
	if w.SyncCtrl != nil {
		// wait for cache synced
		w.SyncCtrl.Run(stop)
	}

	go w.LocalCtrl.Run(stop)

	go w.Server.Run(stop)

	w.Reporter.Run(stop)
}
