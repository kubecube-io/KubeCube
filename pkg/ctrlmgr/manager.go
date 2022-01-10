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

package ctrlmgr

import (
	"context"
	"fmt"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	"github.com/kubecube-io/kubecube/pkg/utils/informer"
	"github.com/kubecube-io/kubecube/pkg/utils/worker"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/kubecube-io/kubecube/pkg/ctrlmgr/webhooks"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"

	"github.com/kubecube-io/kubecube/pkg/clog"

	"github.com/kubecube-io/kubecube/pkg/apis"
	"github.com/kubecube-io/kubecube/pkg/ctrlmgr/controllers"
	"github.com/kubecube-io/kubecube/pkg/utils/exit"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	hnc "sigs.k8s.io/multi-tenancy/incubator/hnc/api/v1alpha2"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apis.AddToScheme(scheme))
	utilruntime.Must(hnc.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
}

type ControllerManager struct {
	*Config

	// CtrlMgr manage all controllers that should be running when as leader
	CtrlMgr ctrl.Manager

	// SubsidiarySyncMgr manage all controllers that should be running as subsidiary
	SubsidiarySyncMgr *multicluster.SyncMgr
}

func NewCtrlMgrWithOpts(options *Config) *ControllerManager {
	cfg := ctrl.GetConfigOrDie()

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                  scheme,
		CertDir:                 options.WebhookCert,
		Port:                    options.WebhookServerPort,
		LeaderElection:          options.LeaderElect,
		MetricsBindAddress:      "0",
		HealthProbeBindAddress:  "0",
		LeaderElectionID:        "kube-cube-manager",
		LeaderElectionNamespace: constants.CubeNamespace,
	})

	if err != nil {
		clog.Fatal("unable to set up controller manager: %v", err)
	}

	syncMgr, err := multicluster.NewSyncMgr(cfg)
	if err != nil {
		clog.Fatal("unable to set up subsidiary sync manager: %v", err)
	}

	syncMgr.Informer.AddEventHandler(informer.NewHandlerOnEvents(syncMgr.OnClusterAdd, syncMgr.OnClusterUpdate, syncMgr.OnClusterDelete))
	syncMgr.Worker = worker.New("cluster", 0, multicluster.ClusterWideKeyFunc, syncMgr.ReconcileCluster)

	return &ControllerManager{Config: options, CtrlMgr: mgr, SubsidiarySyncMgr: syncMgr}
}

func (m *ControllerManager) Initialize() error {
	err := controllers.SetupWithManager(m.CtrlMgr)
	if err != nil {
		return err
	}

	webhooks.SetupWithWebhooks(m.CtrlMgr)

	if err := m.CtrlMgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up health check: %s", err)
	}
	if err := m.CtrlMgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up ready check: %s", err)
	}

	return nil
}

func (m *ControllerManager) Run(stop <-chan struct{}) {
	go func() {
		err := m.CtrlMgr.Start(exit.SetupCtxWithStop(context.Background(), stop))
		if err != nil {
			clog.Fatal("problem run controller manager: %v", err)
		}
	}()

	// after won the leader we need cancel the context
	ctx, cancel := context.WithCancel(context.Background())

	defer func() {
		clog.Info("kubecube won the leader")
		cancel()
	}()

	select {
	case <-m.CtrlMgr.Elected():
		// as elected leader need not multi cluster sync
	case <-time.Tick(20 * time.Second):
		// exceed 10 seconds we thought current mgr is not leader.
		// need cluster sync
		clog.Info("kubecube run as subsidiary")
		go m.SubsidiarySyncMgr.Start(ctx)
		<-m.CtrlMgr.Elected()
	}
}
