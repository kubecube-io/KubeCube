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

package localmgr

import (
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	admisson "sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/kubecube-io/kubecube/pkg/warden/localmgr/controllers/hotplug"
	"github.com/kubecube-io/kubecube/pkg/warden/localmgr/controllers/olm"
	hotplug2 "github.com/kubecube-io/kubecube/pkg/warden/localmgr/webhooks/hotplug"
	"github.com/kubecube-io/kubecube/pkg/warden/localmgr/webhooks/quota"
)

// setupControllersWithManager set up controllers into manager
func setupControllersWithManager(m *LocalManager) error {
	var err error

	err = hotplug.SetupWithManager(m.Manager, m.IsMemberCluster, m.Cluster)
	if err != nil {
		return err
	}

	err = olm.SetupWithManager(m.Manager)
	if err != nil {
		return err
	}

	return nil
}

// setupWithWebhooks set up webhooks into manager
func setupWithWebhooks(m *LocalManager) {
	hookServer := m.GetWebhookServer()

	hookServer.Register("/validate-core-kubernetes-v1-resource-quota", &webhook.Admission{Handler: &quota.ResourceQuotaValidator{PivotClient: m.PivotClient.Direct(), LocalClient: m.GetClient()}})
	hookServer.Register("/warden-validate-hotplug-kubecube-io-v1-hotplug", admisson.ValidatingWebhookFor(hotplug2.NewHotplugValidator(m.IsMemberCluster)))
}
