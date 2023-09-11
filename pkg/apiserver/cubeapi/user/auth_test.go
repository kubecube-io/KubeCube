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

package user_test

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/kubecube-io/kubecube/pkg/apis"
	userv1 "github.com/kubecube-io/kubecube/pkg/apis/user/v1"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/user"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	"github.com/kubecube-io/kubecube/pkg/multicluster/client/fake"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Auth", func() {

	var test123 *userv1.User

	BeforeEach(func() {
		test123 = &userv1.User{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "user.kubecube.io/v1",
				Kind:       "User",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test123",
			},
			Spec: userv1.UserSpec{
				Password: "3f4f95cd5a45bb6c11d8eb2bfbb89642",
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
			Objs:                 []client.Object{test123},
			ClientSetRuntimeObjs: []runtime.Object{},
			Lists:                []client.ObjectList{},
		}
		multicluster.InitFakeMultiClusterMgrWithOpts(opts)
		clients.InitCubeClientSetWithOpts(nil)
	})

	It("login", func() {
		loginBody := user.LoginInfo{Name: "test123", Password: "test123", LoginType: "normal"}
		loginBytes, _ := json.Marshal(loginBody)

		router := gin.New()
		router.POST("/api/v1/cube/login", user.Login)
		w := performRequest(router, http.MethodPost, "/api/v1/cube/login", loginBytes)
		Expect(w.Code).To(Equal(http.StatusOK))
	})
})
