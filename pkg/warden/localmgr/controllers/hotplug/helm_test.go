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

package hotplug_test

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
	"helm.sh/helm/v3/pkg/time"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/warden/localmgr/controllers/hotplug"
)

var _ = Describe("Helm", func() {
	It("init helm", func() {
		// create temporary directory for test case
		var err error
		var dir string
		dir, err = ioutil.TempDir("", "helm-test")
		ioutil.WriteFile(filepath.Join(dir, ".kubeconfig"), []byte(genKubeconfig("test")), 0644)
		Expect(err).NotTo(HaveOccurred())
		// override $HOME/.kube/config
		clientcmd.RecommendedHomeFile = filepath.Join(dir, ".kubeconfig")
		helm := hotplug.NewHelm()
		Expect(helm.K8sConfig).NotTo(BeNil())
		_, err = helm.GetActionConfig("test-ns")
		Expect(err).NotTo(HaveOccurred())
	})

	It("helm list", func() {
		actionConfig := &action.Configuration{
			Releases:       storage.Init(driver.NewMemory()),
			KubeClient:     &kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: ioutil.Discard}},
			Capabilities:   chartutil.DefaultCapabilities,
			RegistryClient: nil,
			Log:            clog.Info,
		}
		helm := hotplug.NewHelm()
		helm.ActionConfig = map[string]*action.Configuration{"test-ns": actionConfig}
		r, err := helm.List("test-ns")
		Expect(err).NotTo(HaveOccurred())
		Expect(len(r)).To(Equal(0))

		release := namedReleaseStub("test-name", release.StatusDeployed)
		actionConfig.Releases.Create(release)
		y, err := helm.List("test-ns")
		Expect(err).NotTo(HaveOccurred())
		Expect(len(y)).To(Equal(1))
		Expect(y[0].Name).To(Equal("test-name"))
	})

	It("helm get values", func() {
		actionConfig := &action.Configuration{
			Releases:       storage.Init(driver.NewMemory()),
			KubeClient:     &kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: ioutil.Discard}},
			Capabilities:   chartutil.DefaultCapabilities,
			RegistryClient: nil,
			Log:            clog.Info,
		}
		helm := hotplug.NewHelm()
		helm.ActionConfig = map[string]*action.Configuration{"test-ns": actionConfig}
		release := namedReleaseStub("test-name", release.StatusDeployed)
		actionConfig.Releases.Create(release)

		y, err := helm.GetValues("test-ns", "test-name")
		Expect(err).NotTo(HaveOccurred())
		r, err := json.Marshal(y)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(r)).To(Equal("{\"testKey\":\"testValue\"}"))
	})

	It("helm uninstall", func() {
		actionConfig := &action.Configuration{
			Releases:       storage.Init(driver.NewMemory()),
			KubeClient:     &kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: ioutil.Discard}},
			Capabilities:   chartutil.DefaultCapabilities,
			RegistryClient: nil,
			Log:            clog.Info,
		}
		helm := hotplug.NewHelm()
		helm.ActionConfig = map[string]*action.Configuration{"test-ns": actionConfig}

		release := namedReleaseStub("test-name", release.StatusDeployed)
		actionConfig.Releases.Create(release)

		err := helm.Uninstall("test-ns", "test-name")
		Expect(err).NotTo(HaveOccurred())
	})

	It("helm install & upgrade", func() {
		actionConfig := &action.Configuration{
			Releases:       storage.Init(driver.NewMemory()),
			KubeClient:     &kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: ioutil.Discard}},
			Capabilities:   chartutil.DefaultCapabilities,
			RegistryClient: nil,
			Log:            clog.Info,
		}
		helm := hotplug.NewHelm()
		helm.ActionConfig = map[string]*action.Configuration{"test-ns": actionConfig}

		r, err := helm.Install("test-ns", "test-name", "", map[string]interface{}{"testKey1": "testValue1"})
		Expect(err).To(HaveOccurred())
		Expect(r).To(BeNil())

		r, err = helm.Upgrade("test-ns", "test-name", "", map[string]interface{}{"testKey2": "testValue2"})
		Expect(err).To(HaveOccurred())
		Expect(r).To(BeNil())
	})

})

func genKubeconfig(contexts ...string) string {
	var sb strings.Builder
	sb.WriteString(`---
apiVersion: v1
kind: Config
clusters:
`)
	for _, ctx := range contexts {
		sb.WriteString(`- cluster:
    server: ` + ctx + `
  name: ` + ctx + `
`)
	}
	sb.WriteString("contexts:\n")
	for _, ctx := range contexts {
		sb.WriteString(`- context:
    cluster: ` + ctx + `
    user: ` + ctx + `
  name: ` + ctx + `
`)
	}

	sb.WriteString("users:\n")
	for _, ctx := range contexts {
		sb.WriteString(`- name: ` + ctx + `
`)
	}
	sb.WriteString("preferences: {}\n")
	if len(contexts) > 0 {
		sb.WriteString("current-context: " + contexts[0] + "\n")
	}

	return sb.String()
}

func namedReleaseStub(name string, status release.Status) *release.Release {
	now := time.Now()
	return &release.Release{
		Name: name,
		Info: &release.Info{
			FirstDeployed: now,
			LastDeployed:  now,
			Status:        status,
			Description:   "Named Release Stub",
		},
		Chart:   buildChart(),
		Config:  map[string]interface{}{"testKey": "testValue"},
		Version: 1,
		Hooks: []*release.Hook{
			{
				Name:     "test-cm",
				Kind:     "ConfigMap",
				Path:     "test-cm",
				Manifest: manifestWithHook,
				Events: []release.HookEvent{
					release.HookPostInstall,
					release.HookPreDelete,
				},
			},
			{
				Name:     "finding-nemo",
				Kind:     "Pod",
				Path:     "finding-nemo",
				Manifest: manifestWithTestHook,
				Events: []release.HookEvent{
					release.HookTest,
				},
			},
		},
	}
}

var manifestWithHook = `kind: ConfigMap
metadata:
  name: test-cm
  annotations:
    "helm.sh/hook": post-install,pre-delete,post-upgrade
data:
  name: value`

var manifestWithTestHook = `kind: Pod
  metadata:
	name: finding-nemo,
	annotations:
	  "helm.sh/hook": test
  spec:
	containers:
	- name: nemo-test
	  image: fake-image
	  cmd: fake-command
  `

func buildChart() *chart.Chart {
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: "v1",
			Name:       "hello",
			Version:    "0.1.0",
		},
		Templates: []*chart.File{
			{Name: "templates/hello", Data: []byte("hello: world")},
			{Name: "templates/goodbye", Data: []byte("goodbye: world")},
			{Name: "templates/empty", Data: []byte("")},
			{Name: "templates/with-partials", Data: []byte(`hello: {{ template "_planet" . }}`)},
			{Name: "templates/partials/_planet", Data: []byte(`{{define "_planet"}}Earth{{end}}`)},
		},
	}
	return c
}
