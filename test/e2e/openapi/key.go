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

package openapi

import (
	"io/ioutil"
	"net/http"

	jsoniter "github.com/json-iterator/go"
	userv1 "github.com/kubecube-io/kubecube/pkg/apis/user/v1"
	"github.com/kubecube-io/kubecube/test/e2e/framework"
	"github.com/onsi/ginkgo"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

var _ = ginkgo.Describe("Test OpenAPI", func() {
	f := framework.NewDefaultFramework("openapi")
	ginkgo.Context("key", func() {
		ginkgo.It("create key", func() {
			url := "/key/create"
			req := f.HttpHelper.Request(http.MethodGet, f.HttpHelper.FormatUrl(url), "")
			resp, err := f.HttpHelper.Client.Do(&req)
			framework.ExpectNoError(err)
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			framework.ExpectNoError(err)
			var result map[string]interface{}
			err = json.Unmarshal(body, &result)
			framework.ExpectNoError(err)
			for k, v := range result {
				switch k {
				case "accessKey", "secretKey":
					framework.ExpectEqual(len(v.(string)), 32)
				case "code":
					framework.ExpectEqual(v.(float64), float64(400))
				case "message":
					framework.ExpectEqual(v.(string), "already have 5 credentials, can't create more.")
				default:
					panic("create key error")
				}
			}
		})

		ginkgo.It("list key and get key token and delete key", func() {
			url := "/key"
			req := f.HttpHelper.Request(http.MethodGet, f.HttpHelper.FormatUrl(url), "")
			resp, err := f.HttpHelper.Client.Do(&req)
			framework.ExpectNoError(err)
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			framework.ExpectNoError(err)
			var keyList userv1.KeyList
			json.Unmarshal(body, &keyList)
			for _, key := range keyList.Items {
				accessKey := key.Name
				secretKey := key.Spec.SecretKey
				url := "/key/token?accessKey=" + accessKey + "&secretKey=" + secretKey
				req := f.HttpHelper.Request(http.MethodGet, f.HttpHelper.FormatUrl(url), "")
				resp, err = f.HttpHelper.Client.Do(&req)
				framework.ExpectNoError(err)
				body, err = ioutil.ReadAll(resp.Body)
				framework.ExpectNoError(err)
				var token map[string]string
				err = json.Unmarshal(body, &token)
				framework.ExpectNoError(err)
				_, ok := token["token"]
				framework.ExpectEqual(ok, true)
			}
			if len(keyList.Items) > 0 {
				url = "/key?accessKey=" + keyList.Items[0].Name
				req = f.HttpHelper.Request(http.MethodDelete, f.HttpHelper.FormatUrl(url), "")
				resp, err = f.HttpHelper.Client.Do(&req)
				framework.ExpectNoError(err)
				body, err = ioutil.ReadAll(resp.Body)
				framework.ExpectNoError(err)
				framework.ExpectEqual("{\"message\":\"success\"}", string(body))
			}
		})
	})
})
