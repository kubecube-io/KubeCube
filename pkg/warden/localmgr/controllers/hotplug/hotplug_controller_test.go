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

package hotplug_test

import (
	"context"
	"io/ioutil"
	"path/filepath"

	hotplugv1 "github.com/kubecube-io/kubecube/pkg/apis/hotplug/v1"
	"github.com/kubecube-io/kubecube/pkg/warden/localmgr/controllers/hotplug"
	"github.com/kubecube-io/kubecube/pkg/warden/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("HotplugController", func() {
	It("test hotplug reconcile", func() {
		// load scheme
		scheme := runtime.NewScheme()
		_ = hotplugv1.AddToScheme(scheme)
		_ = corev1.AddToScheme(scheme)

		// crete
		hotplug1 := hotplugTemplate("common")
		hotplug2 := hotplugTemplate("pivot-cluster-test")
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(&hotplug1, &hotplug2).Build()

		hotplugCtrl := hotplug.HotplugReconciler{}
		hotplugCtrl.Client = fakeClient
		hotplugCtrl.Scheme = scheme

		utils.Cluster = "pivot-cluster-test"

		// override $HOME/.kube/config
		var err error
		var dir string
		dir, err = ioutil.TempDir("", "helm-test")
		ioutil.WriteFile(filepath.Join(dir, ".kubeconfig"), []byte(genKubeconfig("test")), 0644)
		Expect(err).NotTo(HaveOccurred())
		clientcmd.RecommendedHomeFile = filepath.Join(dir, ".kubeconfig")

		ctx := context.Background()
		req := ctrl.Request{}
		req.Name = "common"
		req.NamespacedName = types.NamespacedName{Name: req.Name}
		_, err = hotplugCtrl.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())

		req.Name = "pivot-cluster-test"
		req.NamespacedName = types.NamespacedName{Name: req.Name}
		_, err = hotplugCtrl.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())
	})
})

// tenant template
func hotplugTemplate(name string) hotplugv1.Hotplug {
	return hotplugv1.Hotplug{
		TypeMeta:   metav1.TypeMeta{Kind: "hotplug", APIVersion: "hotplug.kubecube.io/v1"},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: hotplugv1.HotplugSpec{
			Component: []hotplugv1.ComponentConfig{
				{
					Name:   "audit",
					Status: "enabled",
				},
			},
		},
	}
}
