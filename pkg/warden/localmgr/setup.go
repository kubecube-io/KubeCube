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

	"github.com/kubecube-io/kubecube/pkg/warden/localmgr/controllers/crds"
	"github.com/kubecube-io/kubecube/pkg/warden/localmgr/controllers/hotplug"
	project "github.com/kubecube-io/kubecube/pkg/warden/localmgr/controllers/project"
	"github.com/kubecube-io/kubecube/pkg/warden/localmgr/controllers/quota"
	tenant "github.com/kubecube-io/kubecube/pkg/warden/localmgr/controllers/tenant"
	hotplug2 "github.com/kubecube-io/kubecube/pkg/warden/localmgr/webhooks/hotplug"
	project2 "github.com/kubecube-io/kubecube/pkg/warden/localmgr/webhooks/project"
	quota2 "github.com/kubecube-io/kubecube/pkg/warden/localmgr/webhooks/quota"
	tenant2 "github.com/kubecube-io/kubecube/pkg/warden/localmgr/webhooks/tenant"
)

// setupControllersWithManager set up controllers into manager
func setupControllersWithManager(m *LocalManager) error {
	var err error

	err = hotplug.SetupWithManager(m.Manager, m.IsMemberCluster, m.Cluster)
	if err != nil {
		return err
	}

	//err = olm.SetupWithManager(m.Manager)
	//if err != nil {
	//	return err
	//}

	err = tenant.SetupWithManager(m.Manager)
	if err != nil {
		return err
	}

	err = project.SetupWithManager(m.Manager)
	if err != nil {
		return err
	}

	err = crds.SetupWithManager(m.Manager, m.PivotClient.Direct())
	if err != nil {
		return err
	}

	err = quota.SetupWithManager(m.Manager, m.PivotClient.Direct())
	if err != nil {
		return err
	}

	return nil
}

// setupWithWebhooks set up webhooks into manager
func setupWithWebhooks(m *LocalManager) {
	hookServer := m.GetWebhookServer()
	hookServer.Register("/warden-validate-tenant-kubecube-io-v1-tenant", &webhook.Admission{Handler: &tenant2.Validator{Client: m.GetClient(), IsMember: m.IsMemberCluster}})
	hookServer.Register("/warden-validate-tenant-kubecube-io-v1-project", &webhook.Admission{Handler: &project2.Validator{Client: m.GetClient(), IsMember: m.IsMemberCluster}})
	hookServer.Register("/validate-core-kubernetes-v1-resource-quota", &webhook.Admission{Handler: &quota2.ResourceQuotaValidator{PivotClient: m.PivotClient.Direct(), LocalClient: m.GetClient()}})
	hookServer.Register("/warden-validate-hotplug-kubecube-io-v1-hotplug", admisson.ValidatingWebhookFor(hotplug2.NewHotplugValidator(m.IsMemberCluster)))
}
