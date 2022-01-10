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

package resourcemanage_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubecube-io/kubecube/pkg/apis"
	proxy "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/handle"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	"github.com/kubecube-io/kubecube/pkg/multicluster/client/fake"
)

var _ = Describe("Handle", func() {
	var (
		ns1 corev1.Namespace
		ns2 corev1.Namespace
		ns3 corev1.Namespace
	)
	BeforeEach(func() {
		ns1 = corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Namespace",
				APIVersion: "",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:   "ns1",
				Labels: map[string]string{"test1": "1", "test2": "2"},
			},
		}
		ns2 = corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Namespace",
				APIVersion: "",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:   "ns2",
				Labels: map[string]string{"test1": "1", "test2": "test123"},
			},
		}
		ns3 = corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Namespace",
				APIVersion: "",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:   "ns3",
				Labels: map[string]string{"test1": "1"},
			},
		}

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
			Lists:                []client.ObjectList{&corev1.NamespaceList{Items: []corev1.Namespace{ns1, ns2, ns3}}},
		}
		multicluster.InitFakeMultiClusterMgrWithOpts(opts)
		clients.InitCubeClientSetWithOpts(nil)

	})

	It("test filter", func() {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		u, _ := url.Parse("https://example.org/api/v1/cube/clusters/pivot-cluster/api/v1/namespaces?selector=metadata.labels.test1=1,metadata.labels.test2~test&pageSize=10&pageNum=1&sortName=metadata.creatiomTimestamps&sortFunc=time&sortOrder=desc")
		request := http.Request{
			URL:    u,
			Header: http.Header{},
		}
		c.Request = &request
		nsList := corev1.NamespaceList{Items: []corev1.Namespace{ns1, ns2, ns3}}
		nsBytes, err := json.Marshal(nsList)
		Expect(err).To(BeNil())
		retBytes := proxy.Filter(c, nsBytes)
		var ret map[string]interface{}
		err = json.Unmarshal(retBytes, &ret)
		Expect(err).To(BeNil())
		Expect(ret["total"]).To(Equal(float64(1)))
	})

	It("test filter2", func() {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		u, _ := url.Parse("https://example.org/api/v1/cube/clusters/pivot-cluster/api/v1/namespaces?selector=metadata.labels.test1=1,metadata.labels.test2~test&pageSize=10&pageNum=2&sortName=metadata.name&sortFunc=time&sortOrder=asc")
		request := http.Request{
			URL:    u,
			Header: http.Header{},
		}
		c.Request = &request
		nsList := corev1.NamespaceList{Items: []corev1.Namespace{ns1, ns2, ns3}}
		nsBytes, err := json.Marshal(nsList)
		Expect(err).To(BeNil())
		retBytes := proxy.Filter(c, nsBytes)
		var ret map[string]interface{}
		err = json.Unmarshal(retBytes, &ret)
		Expect(err).To(BeNil())
		Expect(ret["total"]).To(Equal(float64(1)))
	})
})
