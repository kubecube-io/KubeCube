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
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubecube-io/kubecube/pkg/apis"
	extend "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/handle"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	"github.com/kubecube-io/kubecube/pkg/multicluster/client/fake"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
)

var _ = Describe("Handle", func() {

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
			Lists:                []client.ObjectList{},
		}
		multicluster.InitFakeMultiClusterMgrWithOpts(opts)
		clients.InitCubeClientSetWithOpts(nil)

	})

	It("extend resourceType deployments, but not allow", func() {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		u, _ := url.Parse("https://example.org?selector=metadata.labels.test1=1&pageSize=10&pageNum=1&sortName=metadata.creatiomTimestamps&sortFunc=time&sortOrder=desc")
		request := http.Request{
			URL:    u,
			Header: http.Header{},
		}
		c.Request = &request
		var params []gin.Param
		params = append(params, gin.Param{Key: "cluster", Value: constants.LocalCluster}, gin.Param{Key: "namespace", Value: "ns1"})
		params = append(params, gin.Param{Key: "resourceType", Value: "deployments"}, gin.Param{Key: "resourceName", Value: "d1"})
		c.Params = params
		extend.ExtendHandle(c)
		Expect(w.Code).To(Equal(403))
		Expect(w.Body.String()).To(Equal("{\"code\":403,\"message\":\"Forbidden.\"}"))
	})
	It("extend resourceType unknown", func() {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		u, _ := url.Parse("https://example.org?selector=metadata.labels.test1=1&pageSize=10&pageNum=1&sortName=metadata.creatiomTimestamps&sortFunc=time&sortOrder=desc")
		request := http.Request{
			URL:    u,
			Header: http.Header{},
		}
		c.Request = &request
		var params []gin.Param
		params = append(params, gin.Param{Key: "cluster", Value: constants.LocalCluster}, gin.Param{Key: "namespace", Value: "ns1"})
		params = append(params, gin.Param{Key: "resourceType", Value: "unknown"}, gin.Param{Key: "resourceName", Value: "c1"})
		c.Params = params
		extend.ExtendHandle(c)
		Expect(w.Code).To(Equal(400))
		Expect(w.Body.String()).To(Equal("{\"code\":400,\"message\":\"resource type param error.\"}"))
	})
	It("extend resourceType right, but httpMethod not match", func() {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		u, _ := url.Parse("https://example.org?selector=metadata.labels.test1=1&pageSize=10&pageNum=1&sortName=metadata.creatiomTimestamps&sortFunc=time&sortOrder=desc")
		request := http.Request{
			URL:    u,
			Header: http.Header{},
		}
		c.Request = &request
		var params []gin.Param
		params = append(params, gin.Param{Key: "cluster", Value: constants.LocalCluster}, gin.Param{Key: "namespace", Value: "ns1"})
		params = append(params, gin.Param{Key: "resourceType", Value: "externalAccess"}, gin.Param{Key: "resourceName", Value: "c1"})
		c.Params = params
		extend.ExtendHandle(c)
		Expect(w.Code).To(Equal(400))
		Expect(w.Body.String()).To(Equal("{\"code\":400,\"message\":\"not match http method.\"}"))
	})
})
