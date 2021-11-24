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

package yamldeploy

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/kubecube-io/kubecube/test/e2e/framework"
	"github.com/onsi/ginkgo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var _ = ginkgo.Describe("Test Yaml Deploy", func() {
	f := framework.NewDefaultFramework("yamldeploy")
	ginkgo.Context("namespace", func() {
		ginkgo.It("yaml deploy create ns", func() {
			yamlStr := `
apiVersion: v1
kind: Namespace
metadata:
  name: yamldeploye2e`
			url := "/extend/clusters/pivot-cluster/yaml/deploy"
			req := f.HttpHelper.Post(f.HttpHelper.FormatUrl(url), yamlStr, nil)
			resp, err := f.HttpHelper.Client.Do(&req)
			framework.ExpectNoError(err)
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			framework.ExpectNoError(err)
			var ns corev1.Namespace
			err = json.Unmarshal(body, &ns)
			if err != nil || ns.Name == "" {
				var ret map[string]interface{}
				err = json.Unmarshal(body, &ret)
				framework.ExpectNoError(err)
				framework.ExpectEqual(ret["code"].(float64), float64(400))
				framework.ExpectEqual(ret["message"], "deploy by yaml fail, %!!(MISSING)!(string=namespaces \"yamldeploye2e\" already exists)v(MISSING)")
			} else {
				framework.ExpectEqual(ns.Name, "yamldeploye2e")
				framework.DeleteNamespace(&ns)
			}
		})

		ginkgo.It("yaml deploy create ns used dryRun param", func() {
			yamlStr := `
apiVersion: v1
kind: Namespace
metadata:
  name: yamldeploye2edryrun`
			url := "/extend/clusters/pivot-cluster/yaml/deploy?dryRun=true"
			req := f.HttpHelper.Post(f.HttpHelper.FormatUrl(url), yamlStr, nil)
			resp, err := f.HttpHelper.Client.Do(&req)
			framework.ExpectNoError(err)
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			framework.ExpectNoError(err)
			var ns corev1.Namespace
			err = json.Unmarshal(body, &ns)
			if err != nil || ns.Name == "" {
				var ret map[string]interface{}
				err = json.Unmarshal(body, &ret)
				framework.ExpectNoError(err)
				framework.ExpectEqual(ret["code"].(float64), float64(400))
				framework.ExpectEqual(ret["message"], "deploy by yaml fail, %!!(MISSING)!(string=namespaces \"yamldeploye2e\" already exists)v(MISSING)")
			} else {
				framework.ExpectEqual(ns.Name, "yamldeploye2edryrun")
			}
		})
	})

	ginkgo.Context("deployment", func() {
		var ns *corev1.Namespace
		var err error
		ginkgo.BeforeEach(func() {
			ns, err = framework.CreateNamespace(f.BaseName)
			framework.ExpectNoError(err)
		})
		ginkgo.AfterEach(func() {
			framework.DeleteNamespace(ns)
		})
		ginkgo.It("yaml deploy create deploy", func() {
			yamlStr := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deploy-e2e-test
  namespace: %s
spec:
  replicas: 4
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.14
        ports:
        - containerPort: 80
`
			yamlStr = fmt.Sprintf(yamlStr, ns.Name)
			url := "/extend/clusters/pivot-cluster/yaml/deploy"
			req := f.HttpHelper.Post(f.HttpHelper.FormatUrl(url), yamlStr, nil)
			resp, err := f.HttpHelper.Client.Do(&req)
			framework.ExpectNoError(err)
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			framework.ExpectNoError(err)
			var dp appsv1.Deployment
			err = json.Unmarshal(body, &dp)
			framework.ExpectNoError(err)
			framework.ExpectEqual(dp.Name, "deploy-e2e-test")
		})
	})

})
