package conversion

import (
	"k8s.io/apimachinery/pkg/runtime"

	// install apis which wanna be converted
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/install"
	admission "k8s.io/kubernetes/pkg/apis/admission/install"
	admissionregistration "k8s.io/kubernetes/pkg/apis/admissionregistration/install"
	apiserverinternal "k8s.io/kubernetes/pkg/apis/apiserverinternal/install"
	apps "k8s.io/kubernetes/pkg/apis/apps/install"
	authentication "k8s.io/kubernetes/pkg/apis/authentication/install"
	authorization "k8s.io/kubernetes/pkg/apis/authorization/install"
	autoscaling "k8s.io/kubernetes/pkg/apis/autoscaling/install"
	batch "k8s.io/kubernetes/pkg/apis/batch/install"
	certificates "k8s.io/kubernetes/pkg/apis/certificates/install"
	coordination "k8s.io/kubernetes/pkg/apis/coordination/install"
	core "k8s.io/kubernetes/pkg/apis/core/install"
	discovery "k8s.io/kubernetes/pkg/apis/discovery/install"
	events "k8s.io/kubernetes/pkg/apis/events/install"
	extensions "k8s.io/kubernetes/pkg/apis/extensions/install"
	flowcontrol "k8s.io/kubernetes/pkg/apis/flowcontrol/install"
	imagepolicy "k8s.io/kubernetes/pkg/apis/imagepolicy/install"
	networking "k8s.io/kubernetes/pkg/apis/networking/install"
	node "k8s.io/kubernetes/pkg/apis/node/install"
	policy "k8s.io/kubernetes/pkg/apis/policy/install"
	rbac "k8s.io/kubernetes/pkg/apis/rbac/install"
	scheduling "k8s.io/kubernetes/pkg/apis/scheduling/install"
	storage "k8s.io/kubernetes/pkg/apis/storage/install"
)

type InstallFunc func(scheme *runtime.Scheme)

func install(scheme *runtime.Scheme, installFuncs ...InstallFunc) {
	for _, fn := range installFuncs {
		fn(scheme)
	}

	admission.Install(scheme)
	admissionregistration.Install(scheme)
	apiserverinternal.Install(scheme)
	apps.Install(scheme)
	authentication.Install(scheme)
	authorization.Install(scheme)
	autoscaling.Install(scheme)
	batch.Install(scheme)
	certificates.Install(scheme)
	coordination.Install(scheme)
	core.Install(scheme)
	discovery.Install(scheme)
	events.Install(scheme)
	extensions.Install(scheme)
	flowcontrol.Install(scheme)
	imagepolicy.Install(scheme)
	networking.Install(scheme)
	node.Install(scheme)
	policy.Install(scheme)
	rbac.Install(scheme)
	scheduling.Install(scheme)
	storage.Install(scheme)
	apiextensions.Install(scheme)
}
