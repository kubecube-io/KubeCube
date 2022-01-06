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
	"bytes"
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/kubecube-io/kubecube/pkg/apis"
	userv1 "github.com/kubecube-io/kubecube/pkg/apis/user/v1"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/user"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	"github.com/kubecube-io/kubecube/pkg/multicluster/client/fake"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"net/http"
	"net/http/httptest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type header struct {
	Key   string
	Value string
}

func performRequest(r http.Handler, method, path string, body []byte, headers ...header) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if body != nil {
		req.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	}
	for _, h := range headers {
		req.Header.Add(h.Key, h.Value)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

var _ = Describe("User", func() {

	var admin *userv1.User

	BeforeEach(func() {
		admin = &userv1.User{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "apiextensions.k8s.io/v1",
				Kind:       "User",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "admin",
			},
			Spec: userv1.UserSpec{
				Password: "admin",
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
			Lists:                []client.ObjectList{&userv1.UserList{Items: []userv1.User{*admin}}},
		}
		multicluster.InitFakeMultiClusterMgrWithOpts(opts)
		clients.InitCubeClientSetWithOpts(nil)
	})

	It("create user", func() {
		userItem := userv1.User{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test123",
			},
			Spec: userv1.UserSpec{
				Password: "test1234",
			},
		}
		userBody, _ := json.Marshal(userItem)

		// create user by api
		router := gin.New()
		router.POST("/api/v1/cube/user", user.CreateUser)
		w := performRequest(router, http.MethodPost, "/api/v1/cube/user", userBody)
		Expect(w.Code).To(Equal(http.StatusOK))

		// get user and check
		cli := clients.Interface().Kubernetes(constants.LocalCluster).Direct()
		user := &userv1.User{}
		err := cli.Get(context.Background(), client.ObjectKey{Name: "test123"}, user)
		Expect(err).To(BeNil())
		Expect(user.Spec.Password).To(Equal("8df43795a53c3761fe6e7cfbda1ff701"))
	})

	It("list user", func() {
		router := gin.New()
		router.GET("/api/v1/cube/user", user.ListUsers)
		w := performRequest(router, http.MethodGet, "/api/v1/cube/user", []byte(""))
		userList := &userv1.UserList{}
		_ = json.Unmarshal(w.Body.Bytes(), userList)
		Expect(w.Code).To(Equal(http.StatusOK))
	})

	It("update user", func() {
		userItem := userv1.User{
			Spec: userv1.UserSpec{
				Password: "abc123456",
			},
		}
		userBody, _ := json.Marshal(userItem)

		// update user by api
		router := gin.New()
		router.PUT("/api/v1/cube/user/:username", user.UpdateUser)
		w := performRequest(router, http.MethodPut, "/api/v1/cube/user/admin", userBody)
		Expect(w.Code).To(Equal(http.StatusOK))

		// get user and check
		cli := clients.Interface().Kubernetes(constants.LocalCluster).Direct()
		user := &userv1.User{}
		err := cli.Get(context.Background(), client.ObjectKey{Name: "admin"}, user)
		Expect(err).To(BeNil())
		Expect(user.Spec.Password).To(Equal("243440cb7dfa88995ba512e4b4d31bcc"))
	})

})
