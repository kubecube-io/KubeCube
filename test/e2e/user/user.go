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

package user

import (
	"context"
	"io/ioutil"
	"net/http"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/onsi/ginkgo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	userv1 "github.com/kubecube-io/kubecube/pkg/apis/user/v1"
	userpkg "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/user"
	"github.com/kubecube-io/kubecube/pkg/clients"
	mgrclient "github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/test/e2e/framework"
)

const (
	waitInterval = 1 * time.Second
	waitTimeout  = 10 * time.Second
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

var _ = ginkgo.Describe("Test user action", func() {
	f := framework.NewDefaultFramework("user")
	ctx := context.Background()

	ginkgo.Context("user", func() {
		var cli mgrclient.Client
		ginkgo.BeforeEach(func() {
			cli = clients.Interface().Kubernetes(constants.LocalCluster)
		})

		ginkgo.It("create user", func() {
			// delete user if exists in cluster
			user := &userv1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test123",
				},
			}
			err := cli.Direct().Delete(ctx, user)

			// create user by api
			userItem := userv1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test123",
				},
				Spec: userv1.UserSpec{
					Password: "test-123",
				},
			}
			userBody, _ := json.Marshal(userItem)
			req := f.HttpHelper.Post(f.HttpHelper.FormatUrl("/user"), string(userBody), nil)
			resp, err := f.HttpHelper.Client.Do(&req)
			framework.ExpectNoError(err)
			framework.ExpectEqual(resp.StatusCode, http.StatusOK)

			// check user is created in cluster
			user = &userv1.User{}
			err = cli.Direct().Get(ctx, client.ObjectKey{Name: "test123"}, user)
			framework.ExpectNoError(err)
			framework.ExpectEqual(user.Spec.Password, "e3a5524bd788dfa149611daa9cc90a8f")

			// check user login by password
			loginBody := userpkg.LoginInfo{Name: "test123", Password: "test-123", LoginType: "normal"}
			loginBytes, _ := json.Marshal(loginBody)
			req = f.HttpHelper.Post(f.HttpHelper.FormatUrl("/login"), string(loginBytes), nil)
			resp, err = f.HttpHelper.Client.Do(&req)
			framework.ExpectNoError(err)
			framework.ExpectEqual(resp.StatusCode, http.StatusOK)
		})

		ginkgo.It("update user", func() {
			// update user by api
			newUser := userv1.User{
				Spec: userv1.UserSpec{
					Phone: "18816212224",
				},
			}
			newBody, _ := json.Marshal(newUser)
			req := f.HttpHelper.Put(f.HttpHelper.FormatUrl("/user/test123"), string(newBody), nil)
			resp, err := f.HttpHelper.Client.Do(&req)
			framework.ExpectEqual(resp.StatusCode, http.StatusOK)
			framework.ExpectNoError(err)

			// check user is updated in cluster
			user := &userv1.User{}
			err = cli.Direct().Get(ctx, client.ObjectKey{Name: "test123"}, user)
			framework.ExpectNoError(err)
			framework.ExpectEqual(user.Spec.Phone, "18816212224")
		})

		ginkgo.It("list user", func() {
			// create user in cluster
			user1 := &userv1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test124",
				},
				Spec: userv1.UserSpec{
					Password: "test-124",
				},
			}
			err := cli.Direct().Delete(ctx, user1)
			err = cli.Direct().Create(ctx, user1)
			framework.ExpectNoError(err)
			err = wait.Poll(waitInterval, waitTimeout,
				func() (bool, error) {
					err = cli.Direct().Get(ctx, client.ObjectKey{Name: "test124"}, user1)
					if err != nil {
						return false, nil
					} else {
						return true, nil
					}
				})
			framework.ExpectNoError(err)

			// list user by api
			req := f.HttpHelper.Get(f.HttpHelper.FormatUrl("/user"), nil)
			resp, err := f.HttpHelper.Client.Do(&req)
			framework.ExpectEqual(resp.StatusCode, http.StatusOK)
			framework.ExpectNoError(err)
			body, err := ioutil.ReadAll(resp.Body)
			framework.ExpectNoError(err)
			var userList userpkg.UserList
			err = json.Unmarshal(body, &userList)
			framework.ExpectNoError(err)
			test124Exist := false
			test123Exist := false
			for _, item := range userList.Items {
				switch item.Name {
				case "test124":
					test124Exist = true
				case "test123":
					test123Exist = true
				default:
					continue
				}
			}
			framework.ExpectEqual(test124Exist, true)
			framework.ExpectEqual(test123Exist, true)
		})
	})
})
