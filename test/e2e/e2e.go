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

package e2e

import (
	"io/ioutil"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/yaml"

	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/yamldeploy"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/test/e2e/framework"
)

var runSuite bool = false

func RunE2ETests(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "E2e Suite")
}

var metadataAccessor = meta.NewAccessor()

var _ = BeforeSuite(func() {
	if !runSuite {
		return
	}
	_ = framework.NewDefaultFramework("e2etest")
	clusters := multicluster.Interface().FuzzyCopy()
	cluster, ok := clusters[constants.PivotCluster]
	if !ok {
		clog.Info("failed to get cluster info")
		return
	}
	yamlFile, err := ioutil.ReadFile("./mock/e2ebefore.yaml")
	if err != nil {
		clog.Info("failed to read yaml file : %v\n", err)
		return
	}
	yamls := strings.Split(string(yamlFile), "---")

	for i := 0; i < 20; i++ {
		ret := false
		for _, y := range yamls {
			if len(y) <= 0 {
				continue
			}
			bodyJson, err := yaml.YAMLToJSON([]byte(y))
			if err != nil {
				clog.Info("failed to parse yaml file : %v\n", err)
				return
			}
			obj, gvk, err := unstructured.UnstructuredJSONScheme.Decode(bodyJson, nil, nil)
			if err != nil {
				clog.Info("%v", err)
				continue
			}
			// new RestClient
			restClient, err := yamldeploy.NewRestClient(cluster.Config, gvk)
			if err != nil {
				clog.Info("%v", err)
				continue
			}

			// init mapping
			restMapping, err := yamldeploy.InitRestMapper(cluster.Client, gvk)
			if err != nil {
				clog.Info("%v", err)
				continue
			}

			// get namespace
			namespace, err := metadataAccessor.Namespace(obj)
			if err != nil {
				clog.Info("%v", err)
				continue
			}

			// create
			restHelper := resource.NewHelper(restClient, restMapping)
			_, err = restHelper.Create(namespace, true, obj)
			if err == nil || errors.IsAlreadyExists(err) {
				ret = true
			} else {
				ret = false
				break
			}
		}
		if ret {
			break
		}
		time.Sleep(time.Duration(5) * time.Second)
	}
})

var _ = AfterSuite(func() {
	if !runSuite {
		return
	}
	_ = framework.NewDefaultFramework("e2etest")
	clusters := multicluster.Interface().FuzzyCopy()
	cluster, ok := clusters[constants.PivotCluster]
	if !ok {
		clog.Info("failed to get cluster info")
		return
	}
	yamlFile, err := ioutil.ReadFile("./mock/e2eafter.yaml")
	if err != nil {
		clog.Info("failed to read yaml file : %v\n", err)
		return
	}
	yamls := strings.Split(string(yamlFile), "---")

	for i := 0; i < 20; i++ {
		ret := false
		for _, y := range yamls {
			if len(y) <= 0 {
				continue
			}
			bodyJson, err := yaml.YAMLToJSON([]byte(y))
			if err != nil {
				clog.Info("failed to parse yaml file : %v\n", err)
				return
			}
			obj, gvk, err := unstructured.UnstructuredJSONScheme.Decode(bodyJson, nil, nil)
			if err != nil {
				clog.Info("%v", err)
				continue
			}
			// new RestClient
			restClient, err := yamldeploy.NewRestClient(cluster.Config, gvk)
			if err != nil {
				clog.Info("%v", err)
				continue
			}

			// init mapping
			restMapping, err := yamldeploy.InitRestMapper(cluster.Client, gvk)
			if err != nil {
				clog.Info("%v", err)
				continue
			}

			// get namespace
			namespace, err := metadataAccessor.Namespace(obj)
			if err != nil {
				clog.Info("%v", err)
				continue
			}
			name, err := metadataAccessor.Name(obj)
			if err != nil {
				clog.Info("%v", err)
				continue
			}

			restHelper := resource.NewHelper(restClient, restMapping)
			_, err = restHelper.Delete(namespace, name)
			if err == nil || errors.IsNotFound(err) {
				ret = true
			} else {
				ret = false
				break
			}
		}
		if ret {
			break
		}
		time.Sleep(time.Duration(5) * time.Second)
	}
})
