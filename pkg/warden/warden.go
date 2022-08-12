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
	"context"

	"k8s.io/client-go/tools/clientcmd"

	"github.com/kubecube-io/kubecube/pkg/clog"
	multiclient "github.com/kubecube-io/kubecube/pkg/multicluster/client"
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
	pivotClient, err := makePivotClient(opts.PivotClusterKubeConfig)
	if err != nil {
		clog.Fatal("init pivot client failed: %v", err)
	}

	w := new(Warden)

	w.Server = &server.Server{
		JwtSecret:              opts.JwtSecret,
		BindAddr:               opts.Addr,
		Port:                   opts.Port,
		TlsKey:                 opts.TlsKey,
		TlsCert:                opts.TlsCert,
		LocalClusterKubeConfig: opts.LocalClusterKubeConfig,
	}

	w.Reporter = &reporter.Reporter{
		Cluster:                opts.Cluster,
		IsWritable:             opts.IsWritable,
		IsMemberCluster:        opts.InMemberCluster,
		PivotCubeHost:          opts.PivotCubeHost,
		PeriodSecond:           opts.PeriodSecond,
		WaitSecond:             opts.WaitSecond,
		LocalClusterKubeConfig: opts.LocalClusterKubeConfig,
		PivotClient:            pivotClient,
	}

	w.LocalCtrl = &localmgr.LocalManager{
		Cluster:                  opts.Cluster,
		IsMemberCluster:          opts.InMemberCluster,
		AllowPrivileged:          opts.AllowPrivileged,
		LeaderElect:              opts.LeaderElect,
		WebhookCert:              opts.WebhookCert,
		WebhookServerPort:        opts.WebhookServerPort,
		PivotClient:              pivotClient,
		NginxNamespace:           opts.NginxNamespace,
		NginxTcpServiceConfigMap: opts.NginxTcpServiceConfigMap,
		NginxUdpServiceConfigMap: opts.NginxUdpServiceConfigMap,
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
	go w.LocalCtrl.Run(stop)

	go w.Server.Run(stop)

	if w.SyncCtrl != nil {
		go w.SyncCtrl.Run(stop)
	}

	w.Reporter.Run(stop)
}

// makePivotClient make client for pivot client
func makePivotClient(kubeconfig string) (multiclient.Client, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	cli, err := multiclient.NewClientFor(context.Background(), cfg)
	if err != nil {
		return nil, err
	}

	return cli, nil
}
