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
package key_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/authentication/v1beta1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubecube-io/kubecube/pkg/apis"
	userv1 "github.com/kubecube-io/kubecube/pkg/apis/user/v1"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/key"
	"github.com/kubecube-io/kubecube/pkg/authentication/authenticators/jwt"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	"github.com/kubecube-io/kubecube/pkg/multicluster/client/fake"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
)

var _ = Describe("Key", func() {
	var (
		userKey  userv1.Key
		testUser userv1.User
	)
	// create ak & sk
	accessKey := key.GetUUID()
	secretKey := key.GetUUID()
	userName := "test"

	BeforeEach(func() {
		testUser = userv1.User{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "apiextensions.k8s.io/v1",
				Kind:       "User",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: userName,
			},
			Spec: userv1.UserSpec{
				Password: userName,
			},
		}
		userKey = userv1.Key{
			TypeMeta: metav1.TypeMeta{
				Kind:       "key",
				APIVersion: "user.kubecube.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: accessKey,
				Labels: map[string]string{
					key.UserLabel: userName,
				},
			},
			Spec: userv1.KeySpec{
				SecretKey: secretKey,
				User:      userName,
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
			Lists:                []client.ObjectList{&userv1.UserList{Items: []userv1.User{testUser}}, &userv1.KeyList{Items: []userv1.Key{userKey}}},
		}
		multicluster.InitFakeMultiClusterMgrWithOpts(opts)
		clients.InitCubeClientSetWithOpts(nil)

	})
	It("test create", func() {
		token, err := jwt.GetAuthJwtImpl().GenerateToken(&v1beta1.UserInfo{Username: "test"})
		Expect(err).To(BeNil())
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		request := http.Request{
			Header: http.Header{},
		}
		request.Header.Add(constants.AuthorizationHeader, "bearer "+token)
		c.Request = &request
		key.CreateKey(c)
		Expect(w.Code).To(Equal(http.StatusOK))
		var m map[string]string
		err2 := json.Unmarshal(w.Body.Bytes(), &m)
		Expect(err2).To(BeNil())
		Expect(m["accessKey"]).NotTo(Equal(""))
		Expect(m["accessKey"]).NotTo(Equal(""))
	})
	It("test delete", func() {
		token, err := jwt.GetAuthJwtImpl().GenerateToken(&v1beta1.UserInfo{Username: "test"})
		Expect(err).To(BeNil())
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		u, _ := url.Parse("https://example.org/?accessKey=" + accessKey)
		request := http.Request{
			URL:    u,
			Header: http.Header{},
		}
		request.Header.Add(constants.AuthorizationHeader, "bearer "+token)
		c.Request = &request
		key.DeleteKey(c)
		Expect(w.Code).To(Equal(http.StatusOK))
	})
	It("test list", func() {
		token, err := jwt.GetAuthJwtImpl().GenerateToken(&v1beta1.UserInfo{Username: "test"})
		Expect(err).To(BeNil())
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		request := http.Request{
			Header: http.Header{},
		}
		request.Header.Add(constants.AuthorizationHeader, "bearer "+token)
		c.Request = &request
		key.ListKey(c)
		Expect(w.Code).To(Equal(http.StatusOK))
		var keyList userv1.KeyList
		err2 := json.Unmarshal(w.Body.Bytes(), &keyList)
		Expect(err2).To(BeNil())
	})
	It("test get token by key", func() {
		token, err := jwt.GetAuthJwtImpl().GenerateToken(&v1beta1.UserInfo{Username: "test"})
		Expect(err).To(BeNil())
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		u, _ := url.Parse("https://example.org/?accessKey=" + accessKey + "&secretKey=" + secretKey)
		request := http.Request{
			URL:    u,
			Header: http.Header{},
		}
		request.Header.Add(constants.AuthorizationHeader, "bearer "+token)
		c.Request = &request
		key.GetTokenByKey(c)
		Expect(w.Code).To(Equal(http.StatusOK))
		var m map[string]string
		err2 := json.Unmarshal(w.Body.Bytes(), &m)
		Expect(err2).To(BeNil())
		Expect(m["token"]).NotTo(Equal(""))
	})
	It("test get token by wrong key", func() {
		token, err := jwt.GetAuthJwtImpl().GenerateToken(&v1beta1.UserInfo{Username: "test"})
		Expect(err).To(BeNil())
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		u, _ := url.Parse("https://example.org/?accessKey=123&secretKey=456")
		request := http.Request{
			URL:    u,
			Header: http.Header{},
		}
		request.Header.Add(constants.AuthorizationHeader, "bearer "+token)
		c.Request = &request
		key.GetTokenByKey(c)
		Expect(w.Code).To(Equal(http.StatusBadRequest))
	})
})
