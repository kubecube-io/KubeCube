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
package deployment_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kubecube-io/kubecube/pkg/apis"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/deployment"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	"github.com/kubecube-io/kubecube/pkg/multicluster/client/fake"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
)

var _ = Describe("Deployment", func() {
	var (
		ns        = "namespace-test"
		dp        appsv1.Deployment
		dpList    appsv1.DeploymentList
		rs        appsv1.ReplicaSet
		rsList    appsv1.ReplicaSetList
		pod       corev1.Pod
		podList   corev1.PodList
		event     corev1.Event
		eventList corev1.EventList
	)
	BeforeEach(func() {
		dp = appsv1.Deployment{
			TypeMeta:   metav1.TypeMeta{Kind: "Deployment", APIVersion: "apps/v1"},
			ObjectMeta: metav1.ObjectMeta{Name: "dp", Namespace: ns, UID: "dpid"},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "testapp"},
				},
			},
		}
		dpList = appsv1.DeploymentList{
			Items: []appsv1.Deployment{dp},
		}
		rs = appsv1.ReplicaSet{
			TypeMeta: metav1.TypeMeta{Kind: "ReplicasSet", APIVersion: "apps/v1"},
			ObjectMeta: metav1.ObjectMeta{
				Name:            "rs",
				Namespace:       ns,
				Labels:          map[string]string{"app": "testapp"},
				UID:             "rsid",
				OwnerReferences: []metav1.OwnerReference{{UID: "dpid"}},
			},
		}
		rsList = appsv1.ReplicaSetList{
			Items: []appsv1.ReplicaSet{rs},
		}
		pod = corev1.Pod{
			TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{
				Name:            "pod",
				Namespace:       ns,
				Labels:          map[string]string{"app": "testapp"},
				UID:             "podid",
				OwnerReferences: []metav1.OwnerReference{{UID: "rsid"}},
			},
			Status: corev1.PodStatus{
				Phase: "Running",
			},
		}
		podList = corev1.PodList{
			Items: []corev1.Pod{pod},
		}
		event = corev1.Event{
			TypeMeta:       metav1.TypeMeta{Kind: "Event", APIVersion: "v1"},
			ObjectMeta:     metav1.ObjectMeta{Name: "event", Namespace: ns},
			InvolvedObject: corev1.ObjectReference{UID: "podid"},
			Type:           "Warning",
		}
		eventList = corev1.EventList{Items: []corev1.Event{event}}
	})
	JustBeforeEach(func() {
		scheme := runtime.NewScheme()
		apis.AddToScheme(scheme)
		corev1.AddToScheme(scheme)
		appsv1.AddToScheme(scheme)
		coordinationv1.AddToScheme(scheme)
		opts := &fake.Options{
			Scheme:               scheme,
			Objs:                 []client.Object{},
			ClientSetRuntimeObjs: []runtime.Object{},
			Lists:                []client.ObjectList{&dpList, &rsList, &podList, &eventList},
		}
		multicluster.InitFakeMultiClusterMgrWithOpts(opts)
		clients.InitCubeClientSetWithOpts(nil)
	})

	It("test get deployment extend info", func() {
		client := clients.Interface().Kubernetes(constants.LocalCluster)
		Expect(client).NotTo(BeNil())
		deploy := deployment.NewDeployment(client, ns, resources.Filter{Limit: 10})
		ret := deploy.GetExtendDeployments()
		Expect(ret["total"]).To(Equal(1))
		items := ret["items"]
		dpInfo := items.([]interface{})[0].(map[string]interface{})
		dpInfoname := dpInfo["metadata"].(metav1.ObjectMeta).Name
		Expect(dpInfoname).To(Equal("dp"))
		podStatus := dpInfo["podStatus"].(map[string]interface{})
		Expect(podStatus["running"]).To(Equal(1))
	})
})
