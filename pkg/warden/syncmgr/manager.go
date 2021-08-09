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

package syncmgr

import (
	"context"
	"fmt"
	"net/http"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"sigs.k8s.io/multi-tenancy/incubator/hnc/api/v1alpha2"

	"github.com/kubecube-io/kubecube/pkg/warden/utils"

	"github.com/kubecube-io/kubecube/pkg/clog"

	"github.com/kubecube-io/kubecube/pkg/warden/reporter"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	//v1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubecube-io/kubecube/pkg/apis"
	"github.com/kubecube-io/kubecube/pkg/utils/exit"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
)

const healthProbeAddr = "0.0.0.0:9777"

var (
	log clog.CubeLogger

	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(apis.AddToScheme(scheme))

	utilruntime.Must(v1alpha2.AddToScheme(scheme))

	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
}

// SyncManager watch and sync resource from cluster master to cluster member
type SyncManager struct {
	ctrl.Manager
	LocalClient            client.Client
	PivotClusterKubeConfig string
}

func (s *SyncManager) Initialize() error {
	log = clog.WithName("syncmgr")

	cfg, err := clientcmd.BuildConfigFromFlags("", s.PivotClusterKubeConfig)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %s", err.Error())
	}

	s.Manager, err = manager.New(cfg, ctrl.Options{Scheme: scheme, HealthProbeBindAddress: healthProbeAddr})
	if err != nil {
		return fmt.Errorf("error new sync mgr: %s", err.Error())
	}

	// todo(weilaaa): init pivot client here is not elegant
	utils.PivotClient = s.Manager.GetClient()

	s.LocalClient, err = client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("error new local client: %s", err.Error())
	}

	for _, r := range syncResources {
		err = s.setupCtrlWithManager(r)
		if err != nil {
			return err
		}
	}

	err = s.Manager.AddReadyzCheck("readyz", healthz.Ping)
	if err != nil {
		return err
	}

	reporter.RegisterCheckFunc(s.readyzCheck)

	return nil
}

func (s *SyncManager) readyzCheck() bool {
	path := fmt.Sprintf("http://%s/readyz", healthProbeAddr)

	resp, err := http.Get(path)
	if err != nil {
		log.Debug("sync manager not ready: %v", err)
		return false
	}

	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	log.Info("sync manager ready")

	return true
}

func (s *SyncManager) Run(stop <-chan struct{}) {
	ctx := exit.SetupCtxWithStop(context.Background(), stop)
	err := s.Manager.Start(ctx)
	if err != nil {
		log.Fatal("start sync manager failed: %s", err)
	}
}
