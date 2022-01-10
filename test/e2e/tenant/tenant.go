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

package tenant

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"strconv"

	tenantv1 "github.com/kubecube-io/kubecube/pkg/apis/tenant/v1"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/test/e2e/framework"
	"github.com/onsi/ginkgo"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	hnc "sigs.k8s.io/multi-tenancy/incubator/hnc/api/v1alpha2"
)

var _ = ginkgo.Describe("Test Tenant and Project", func() {
	f := framework.NewDefaultFramework("tenant")
	ginkgo.Context("tenant and project", func() {
		randnum := strconv.Itoa(rand.Intn(10000))
		var tenantName = fmt.Sprintf("e2etest-tenant-%s", randnum)
		var projectName = fmt.Sprintf("e2etest-project-%s", randnum)
		var cli client.Client
		ginkgo.BeforeEach(func() {
			cli = clients.Interface().Kubernetes(constants.LocalCluster)
		})

		ginkgo.It("create tenant", func() {

			tenantJson := "{\"apiVersion\":\"tenant.kubecube.io/v1\",\"kind\":\"Tenant\",\"metadata\":{\"name\":\"" + tenantName + "\"},\"spec\":{\"displayName\":\"my-tenant\",\"description\":\"my-tenant\"}}"
			url := "/proxy/clusters/pivot-cluster/apis/tenant.kubecube.io/v1/tenants"
			req := f.HttpHelper.Post(f.HttpHelper.FormatUrl(url), tenantJson, nil)
			_, err := f.HttpHelper.Client.Do(&req)
			framework.ExpectNoError(err)
		})

		ginkgo.It("create project without tenant", func() {
			projectJson := "{\"apiVersion\":\"tenant.kubecube.io/v1\",\"kind\":\"Project\",\"metadata\":{\"name\":\"project-sample\"},\"spec\":{\"description\":\"my-project\",\"displayName\":\"my-project\"}}"
			url := "/proxy/clusters/pivot-cluster/apis/tenant.kubecube.io/v1/projects"
			req := f.HttpHelper.Post(f.HttpHelper.FormatUrl(url), projectJson, nil)
			resp, err := f.HttpHelper.Client.Do(&req)
			framework.ExpectNoError(err)
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			framework.ExpectNoError(err)
			var statusErr metav1.Status
			err = json.Unmarshal(body, &statusErr)
			framework.ExpectNoError(err)
			framework.ExpectEqual(int32(403), statusErr.Code)
			framework.ExpectEqual("can not find .metadata.labels.kubecube.io/tenant label", string(statusErr.Reason))
		})

		ginkgo.It("create project within tenant", func() {
			projectJson := "{\"apiVersion\":\"tenant.kubecube.io/v1\",\"kind\":\"Project\",\"metadata\":{\"labels\":{\"kubecube.io/tenant\":\"" + tenantName + "\"},\"name\":\"" + projectName + "\"},\"spec\":{\"description\":\"my-project\",\"displayName\":\"my-project\"}}"
			url := "/proxy/clusters/pivot-cluster/apis/tenant.kubecube.io/v1/projects"
			req := f.HttpHelper.Post(f.HttpHelper.FormatUrl(url), projectJson, nil)
			_, err := f.HttpHelper.Client.Do(&req)
			framework.ExpectNoError(err)
		})

		ginkgo.It("has namespace in tenant.spec and project.spec", func() {
			err := wait.Poll(f.Timeouts.WaitInterval, f.Timeouts.WaitTimeout,
				func() (bool, error) {
					var tenant tenantv1.Tenant
					err := cli.Direct().Get(context.TODO(), types.NamespacedName{Name: tenantName}, &tenant)
					if err != nil {
						return false, nil
					} else {
						framework.ExpectEqual("kubecube-tenant-"+tenantName, tenant.Spec.Namespace)
						return true, nil
					}
				})
			framework.ExpectNoError(err)

			err = wait.Poll(f.Timeouts.WaitInterval, f.Timeouts.WaitTimeout,
				func() (bool, error) {
					var project tenantv1.Project
					err := cli.Direct().Get(context.TODO(), types.NamespacedName{Name: projectName}, &project)
					if err != nil {
						return false, nil
					} else {
						framework.ExpectEqual("kubecube-project-"+projectName, project.Spec.Namespace)
						return true, nil
					}
				})
			framework.ExpectNoError(err)
		})

		ginkgo.It("delete project before delete .spec.namespace", func() {
			url := "/proxy/clusters/pivot-cluster/apis/tenant.kubecube.io/v1/projects/" + projectName
			req := f.HttpHelper.Delete(f.HttpHelper.FormatUrl(url))
			resp, err := f.HttpHelper.Client.Do(&req)
			framework.ExpectNoError(err)
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			framework.ExpectNoError(err)
			var statusErr metav1.Status
			err = json.Unmarshal(body, &statusErr)
			framework.ExpectNoError(err)
			framework.ExpectEqual(int32(403), statusErr.Code)
		})

		ginkgo.It("delete project after delete .spec.namespace", func() {
			// delete subnamespace
			url := fmt.Sprintf("/proxy/clusters/pivot-cluster/apis/hnc.x-k8s.io/v1alpha2/namespaces/%s/subnamespaceanchors/%s", "kubecube-tenant-"+tenantName, "kubecube-project-"+projectName)
			req := f.HttpHelper.Delete(f.HttpHelper.FormatUrl(url))
			r, err := f.HttpHelper.Client.Do(&req)
			framework.ExpectNoError(err)
			b, err := ioutil.ReadAll(r.Body)
			framework.ExpectNoError(err)
			var subns hnc.SubnamespaceAnchor
			err = json.Unmarshal(b, &subns)
			framework.ExpectNoError(err)
			framework.ExpectEqual("kubecube-project-"+projectName, subns.Name)
			// wait namespace deleted
			err = wait.Poll(f.Timeouts.WaitInterval, f.Timeouts.WaitTimeout, func() (bool, error) {
				var ns v1.Namespace
				err := cli.Direct().Get(context.TODO(), types.NamespacedName{Name: "kubecube-project-" + projectName}, &ns)
				if err != nil && apierrors.IsNotFound(err) {
					return true, nil
				}
				return false, nil
			})
			framework.ExpectNoError(err)
			url = "/proxy/clusters/pivot-cluster/apis/tenant.kubecube.io/v1/projects/" + projectName
			req = f.HttpHelper.Delete(f.HttpHelper.FormatUrl(url))
			resp, err := f.HttpHelper.Client.Do(&req)
			framework.ExpectNoError(err)
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			framework.ExpectNoError(err)
			var status metav1.Status
			err = json.Unmarshal(body, &status)
			framework.ExpectNoError(err)
			framework.ExpectEqual("Success", status.Status)
		})

		ginkgo.It("delete tenant before delete .spec.namespace", func() {
			url := "/proxy/clusters/pivot-cluster/apis/tenant.kubecube.io/v1/tenants/" + tenantName
			req := f.HttpHelper.Delete(f.HttpHelper.FormatUrl(url))
			resp, err := f.HttpHelper.Client.Do(&req)
			framework.ExpectNoError(err)
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			framework.ExpectNoError(err)
			var statusErr metav1.Status
			err = json.Unmarshal(body, &statusErr)
			framework.ExpectNoError(err)
			framework.ExpectEqual(int32(403), statusErr.Code)
		})

		ginkgo.It("delete tenant after delete .spec.namespace", func() {
			var ns v1.Namespace
			err := cli.Direct().Get(context.TODO(), types.NamespacedName{Name: "kubecube-tenant-" + tenantName}, &ns)
			framework.ExpectNoError(err)
			err = framework.DeleteNamespace(&ns)
			framework.ExpectNoError(err)
			url := "/proxy/clusters/pivot-cluster/apis/tenant.kubecube.io/v1/tenants/" + tenantName
			req := f.HttpHelper.Delete(f.HttpHelper.FormatUrl(url))
			resp, err := f.HttpHelper.Client.Do(&req)
			framework.ExpectNoError(err)
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			framework.ExpectNoError(err)
			var status metav1.Status
			err = json.Unmarshal(body, &status)
			framework.ExpectNoError(err)
			framework.ExpectEqual("Success", status.Status)
		})
	})

})
