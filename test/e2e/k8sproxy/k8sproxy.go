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

package k8sproxy

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	jsoniter "github.com/json-iterator/go"
	"github.com/kubecube-io/kubecube/test/e2e/framework"
	"github.com/onsi/ginkgo/v2"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

var _ = ginkgo.Describe("Test k8s proxy", func() {
	f := framework.NewDefaultFramework("k8sproxy")

	ginkgo.Context("Test namespace", func() {
		ginkgo.It("get namespace", func() {
			req := f.HttpHelper.Get(f.HttpHelper.FormatUrl("/proxy/clusters/pivot-cluster/api/v1/namespaces?pageSize=1000"), nil)
			resp, err := f.HttpHelper.Client.Do(&req)
			framework.ExpectNoError(err)
			body, err := ioutil.ReadAll(resp.Body)
			framework.ExpectNoError(err)
			var nsList v1.NamespaceList
			err = json.Unmarshal(body, &nsList)
			framework.ExpectNoError(err)
			defaultName := ""
			kubeSystemName := ""
			for _, ns := range nsList.Items {
				if ns.Name == "default" {
					defaultName = ns.Name
				}
				if ns.Name == "kube-system" {
					kubeSystemName = ns.Name
				}
			}
			framework.ExpectEqual(defaultName, "default")
			framework.ExpectEqual(kubeSystemName, "kube-system")
		})

	})

	ginkgo.Context("Test deployment", func() {
		var ns *v1.Namespace
		ginkgo.BeforeEach(func() {
			ns, _ = framework.CreateNamespace(context.Background(), f.BaseName)
		})
		ginkgo.AfterEach(func() {
			_ = framework.DeleteNamespace(ns)
		})
		ginkgo.Context("", func() {
			ginkgo.It("post deployment", func() {
				deployJosn := "{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"name\":\"deployment-example\"},\"spec\":{\"replicas\":3,\"revisionHistoryLimit\":10,\"selector\":{\"matchLabels\":{\"app\":\"nginx\"}},\"template\":{\"metadata\":{\"labels\":{\"app\":\"nginx\"}},\"spec\":{\"containers\":[{\"name\":\"nginx\",\"image\":\"nginx:1.14\",\"ports\":[{\"containerPort\":80}]}]}}}}"
				url := fmt.Sprintf("/proxy/clusters/pivot-cluster/apis/apps/v1/namespaces/%s/deployments", ns.Name)
				req := f.HttpHelper.Post(f.HttpHelper.FormatUrl(url), deployJosn, nil)
				resp, err := f.HttpHelper.Client.Do(&req)
				framework.ExpectNoError(err)
				defer resp.Body.Close()
				body, err := ioutil.ReadAll(resp.Body)
				framework.ExpectNoError(err)
				var deploy appsv1.Deployment
				err = json.Unmarshal(body, &deploy)
				framework.ExpectNoError(err)
				framework.ExpectEqual("deployment-example", deploy.Name)
			})

			ginkgo.It("post illegal deployment", func() {
				deployJosn := "{\"apiVersion\":\"apps/v1\",\"kind\":\"Deploymentxxx\",\"metadata\":{\"name\":\"deployment-example\"},\"spec\":{\"replicas\":3,\"revisionHistoryLimit\":10,\"selector\":{\"matchLabels\":{\"app\":\"nginx\"}},\"template\":{\"metadata\":{\"labels\":{\"app\":\"nginx\"}},\"spec\":{\"containers\":[{\"name\":\"nginx\",\"image\":\"nginx:1.14\",\"ports\":[{\"containerPort\":80}]}]}}}}"
				url := fmt.Sprintf("/proxy/clusters/pivot-cluster/apis/apps/v1/namespaces/%s/deployments", ns.Name)
				req := f.HttpHelper.Post(f.HttpHelper.FormatUrl(url), deployJosn, nil)
				resp, err := f.HttpHelper.Client.Do(&req)
				framework.ExpectNoError(err)
				defer resp.Body.Close()
				body, err := ioutil.ReadAll(resp.Body)
				framework.ExpectNoError(err)
				var statusErr metav1.Status
				err = json.Unmarshal(body, &statusErr)
				framework.ExpectNoError(err)
				framework.ExpectEqual("Status", statusErr.Kind)
				framework.ExpectEqual(int32(400), statusErr.Code)
				framework.ExpectEqual("Failure", statusErr.Status)
			})
		})

	})

	ginkgo.Context("Test multi user create deploy", func() {
		ginkgo.It("get namespace default", func() {
			deployJosn := "{\"apiVersion\":\"apps/v1\",\"kind\":\"Deploymentxxx\",\"metadata\":{\"name\":\"deployment-example\"},\"spec\":{\"replicas\":3,\"revisionHistoryLimit\":10,\"selector\":{\"matchLabels\":{\"app\":\"nginx\"}},\"template\":{\"metadata\":{\"labels\":{\"app\":\"nginx\"}},\"spec\":{\"containers\":[{\"name\":\"nginx\",\"image\":\"nginx:1.14\",\"ports\":[{\"containerPort\":80}]}]}}}}"
			url := "/proxy/clusters/pivot-cluster/apis/apps/v1/namespaces/e2e-ns/deployments"

			ret := f.HttpHelper.MultiUserRequest(http.MethodPost, url, deployJosn, nil)
			for k, v := range ret {
				framework.ExpectNoError(v.Err)
				b, err := ioutil.ReadAll(v.Resp.Body)
				framework.ExpectNoError(err)
				var statusErr metav1.Status
				err = json.Unmarshal(b, &statusErr)
				framework.ExpectNoError(err)
				switch k {
				case "admin":
					framework.ExpectEqual(int32(400), statusErr.Code)
					framework.ExpectEqual(string(statusErr.Reason), "BadRequest")
				case "tenantAdmin":
					framework.ExpectEqual(int32(400), statusErr.Code)
					framework.ExpectEqual(string(statusErr.Reason), "BadRequest")
				case "projectAdmin":
					framework.ExpectEqual(int32(400), statusErr.Code)
					framework.ExpectEqual(string(statusErr.Reason), "BadRequest")
				case "user":
					framework.ExpectEqual(int32(403), statusErr.Code)
					framework.ExpectEqual(string(statusErr.Reason), "Forbidden")
				}
			}
		})
	})
})
