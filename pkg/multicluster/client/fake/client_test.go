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

package fake_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mgrclient "github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"github.com/kubecube-io/kubecube/pkg/multicluster/client/fake"
)

const trackerAddResourceVersion = "999"

var _ = Describe("Fake cube client", func() {
	var dep *appsv1.Deployment
	var dep2 *appsv1.Deployment
	var cm *corev1.ConfigMap
	var cli mgrclient.Client

	BeforeEach(func() {
		dep = &appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            "test-deployment",
				Namespace:       "ns1",
				ResourceVersion: trackerAddResourceVersion,
			},
		}
		dep2 = &appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-2",
				Namespace: "ns1",
				Labels: map[string]string{
					"test-label": "label-value",
				},
				ResourceVersion: trackerAddResourceVersion,
			},
		}
		cm = &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            "test-cm",
				Namespace:       "ns2",
				ResourceVersion: trackerAddResourceVersion,
			},
			Data: map[string]string{
				"test-key": "test-value",
			},
		}
	})

	AssertClientBehavior := func() {
		It("Direct() should able to create", func() {
			By("Creating a new configmap")
			newcm := &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "new-test-cm",
					Namespace: "ns2",
				},
			}
			err := cli.Direct().Create(context.Background(), newcm)
			Expect(err).To(BeNil())

			By("Getting the new configmap")
			namespacedName := types.NamespacedName{
				Name:      "new-test-cm",
				Namespace: "ns2",
			}
			obj := &corev1.ConfigMap{}
			err = cli.Direct().Get(context.Background(), namespacedName, obj)
			Expect(err).To(BeNil())
			Expect(obj).To(Equal(newcm))
			Expect(obj.ObjectMeta.ResourceVersion).To(Equal("1"))
		})

		It("cache() should be able to List", func() {
			By("Listing all deployments in a namespace")
			list := &appsv1.DeploymentList{}
			err := cli.Cache().List(context.Background(), list, client.InNamespace("ns1"))
			Expect(err).To(BeNil())
			Expect(list.Items).To(HaveLen(2))
			Expect(list.Items).To(ConsistOf(*dep, *dep2))
		})

		It("ClientSet() should be able to Get", func() {
			By("Getting a deployment")
			obj, err := cli.ClientSet().AppsV1().Deployments("ns1").Get(context.Background(), "test-deployment", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(obj).To(Equal(dep))
		})
	}

	Context("with given scheme", func() {
		BeforeEach(func(done Done) {
			scheme := runtime.NewScheme()
			Expect(corev1.AddToScheme(scheme)).To(Succeed())
			Expect(appsv1.AddToScheme(scheme)).To(Succeed())
			Expect(coordinationv1.AddToScheme(scheme)).To(Succeed())
			opts := &fake.Options{
				Scheme:               scheme,
				Objs:                 []client.Object{cm},
				ClientSetRuntimeObjs: []runtime.Object{dep},
				Lists:                []client.ObjectList{&appsv1.DeploymentList{Items: []appsv1.Deployment{*dep, *dep2}}},
			}
			cli = fake.NewFakeClients(opts)
			close(done)
		})
		AssertClientBehavior()
	})
})
