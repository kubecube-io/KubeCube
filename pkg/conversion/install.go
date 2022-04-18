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

package conversion

import (
	"k8s.io/apimachinery/pkg/runtime"

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

	// install apis which wanna be converted

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
