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
	hookServer.Register("/warden-validate-hotplug-kubecube-io-v1-hotplug", admisson.ValidatingWebhookFor(hotplug2.NewHotplugValidator()))
}
