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

package utils

import (
	"github.com/kubecube-io/kubecube/pkg/apis"
	"github.com/kubecube-io/kubecube/pkg/clog"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/multi-tenancy/incubator/hnc/api/v1alpha2"
)

// PivotClient has the way to communicate with pivot cluster
var PivotClient client.Client

// InitPivotClient make sure pivot initialized
func InitPivotClient() {
	if PivotClient == nil {
		// below logic should execute in pivot cluster
		scheme := runtime.NewScheme()
		utilruntime.Must(clientgoscheme.AddToScheme(scheme))
		utilruntime.Must(apis.AddToScheme(scheme))
		utilruntime.Must(v1alpha2.AddToScheme(scheme))
		utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
		p, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
		if err != nil {
			clog.Fatal("pivot client init failed: %v", err)
		}
		PivotClient = p
	}
}

// Cluster local cluster name
var Cluster string

// local namespace
var Namespace string

// feature configmap
var FeatureConfigMap string
