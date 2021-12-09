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

package webhooks

import (
	"github.com/kubecube-io/kubecube/pkg/warden/localmgr/webhooks/hotplug"
	"github.com/kubecube-io/kubecube/pkg/warden/localmgr/webhooks/quota"
	"github.com/kubecube-io/kubecube/pkg/warden/utils"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	admisson "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// SetupWithWebhooks set up webhooks into manager
func SetupWithWebhooks(mgr manager.Manager) {
	hookServer := mgr.GetWebhookServer()

	hookServer.Register("/validate-core-kubernetes-v1-resource-quota", &webhook.Admission{Handler: &quota.ResourceQuotaValidator{PivotClient: utils.PivotClient, LocalClient: mgr.GetClient()}})

	hookServer.Register("/warden-validate-hotplug-kubecube-io-v1-hotplug", admisson.ValidatingWebhookFor(hotplug.NewHotplugValidator()))

}
