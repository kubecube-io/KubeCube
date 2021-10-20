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

package localmgr

import (
	"context"
	"fmt"
	"net/http"

	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/warden/localmgr/controllers"
	"github.com/kubecube-io/kubecube/pkg/warden/localmgr/webhooks"
	"github.com/kubecube-io/kubecube/pkg/warden/reporter"

	"github.com/kubecube-io/kubecube/pkg/apis"
	"github.com/kubecube-io/kubecube/pkg/utils/exit"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

const healthProbeAddr = "0.0.0.0:9778"

var (
	log clog.CubeLogger

	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(apis.AddToScheme(scheme))

	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))

	utilruntime.Must(operatorsv1.AddToScheme(scheme))
}

// LocalManager is used to list and watch the resource of local
// cluster, also has the way to register webhook
type LocalManager struct {
	AllowPrivileged   bool
	LeaderElect       bool
	WebhookCert       string
	WebhookServerPort int

	ctrl.Manager
}

func (m *LocalManager) Initialize() error {
	log = clog.WithName("localmgr")

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  scheme,
		CertDir:                 m.WebhookCert,
		Port:                    m.WebhookServerPort,
		LeaderElection:          m.LeaderElect,
		LeaderElectionID:        "kube-cube-warden-local-manager",
		LeaderElectionNamespace: constants.CubeNamespace,
		HealthProbeBindAddress:  healthProbeAddr,
		MetricsBindAddress:      "0",
	})

	if err != nil {
		return err
	}

	m.Manager = mgr

	err = controllers.SetupWithManager(m.Manager)
	if err != nil {
		return err
	}

	webhooks.SetupWithWebhooks(m.Manager)

	err = m.Manager.AddReadyzCheck("readyz", healthz.Ping)
	if err != nil {
		return err
	}

	reporter.RegisterCheckFunc(m.readyzCheck)

	return nil
}

func (m *LocalManager) readyzCheck() bool {
	path := fmt.Sprintf("http://%s/readyz", healthProbeAddr)

	resp, err := http.Get(path)
	if err != nil {
		log.Debug("local controller manager not ready: %v", err)
		return false
	}

	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	log.Info("local controller manager ready")

	return true
}

func (m *LocalManager) Run(stop <-chan struct{}) {
	ctx := exit.SetupCtxWithStop(context.Background(), stop)
	err := m.Manager.Start(ctx)
	if err != nil {
		log.Fatal("start local controller manager failed: %s", err)
	}
}
