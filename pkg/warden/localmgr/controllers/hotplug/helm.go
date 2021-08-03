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

package hotplug

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/kubecube-io/kubecube/pkg/clog"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/homedir"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Helm struct {
	K8sConfig    *rest.Config
	ActionConfig map[string]*action.Configuration
}

func NewHelm() *Helm {

	config, err := ctrl.GetConfig()
	if err != nil {
		clog.Error("can not get kubeconfig from local or serviceAccount")
	}

	return &Helm{
		K8sConfig:    config,
		ActionConfig: make(map[string]*action.Configuration),
	}
}

// get action config
func (h *Helm) GetActionConfig(namespace string) (*action.Configuration, error) {
	if c, ok := h.ActionConfig[namespace]; ok {
		return c, nil
	}
	// init action config
	actionConfig := new(action.Configuration)
	kubeConfig := genericclioptions.NewConfigFlags(false)
	kubeConfig.APIServer = &h.K8sConfig.Host
	kubeConfig.BearerToken = &h.K8sConfig.BearerToken
	kubeConfig.CAFile = &h.K8sConfig.CAFile
	kubeConfig.Namespace = &namespace
	log := clog.WithName("hotplug-helm")
	err := actionConfig.Init(kubeConfig, namespace, os.Getenv("HELM_DRIVER"), log.Info)
	if err != nil {
		clog.Error("can not create helm action config")
		return nil, err
	}
	h.ActionConfig[namespace] = actionConfig
	return actionConfig, nil
}

// helm list
func (h *Helm) List(namespace string) ([]*release.Release, error) {
	actionConfig, err := h.GetActionConfig(namespace)
	if err != nil {
		return nil, err
	}
	listAction := action.NewList(actionConfig)
	return listAction.Run()
}

// helm status
func (h *Helm) Status(namespace string, name string) (*release.Release, error) {
	actionConfig, err := h.GetActionConfig(namespace)
	if err != nil {
		return nil, err
	}
	statusAction := action.NewStatus(actionConfig)
	return statusAction.Run(name)
}

// helm get values xxx
func (h *Helm) GetValues(namespace string, name string) (map[string]interface{}, error) {
	actionConfig, err := h.GetActionConfig(namespace)
	if err != nil {
		return nil, err
	}
	getValueAction := action.NewGetValues(actionConfig)
	return getValueAction.Run(name)
}

// helm install
func (h *Helm) Install(namespace, name, pkgName string, envs map[string]interface{}) (*release.Release, error) {
	actionConfig, err := h.GetActionConfig(namespace)
	if err != nil {
		return nil, err
	}
	file := ""
	if home := homedir.HomeDir(); home != "" {
		file = filepath.Join(home, "helmchartpkg", pkgName)
	} else {
		currentPath, _ := os.Getwd()
		file = filepath.Join(currentPath, "helmchartpkg", pkgName)
	}
	c, err := loader.LoadFile(file)
	if err != nil {
		return nil, err
	}
	installAction := action.NewInstall(actionConfig)
	installAction.ReleaseName = name
	installAction.Namespace = namespace
	installAction.CreateNamespace = true
	relaese, err := installAction.Run(c, envs)
	if err != nil {
		clog.Info("can not install the release, %v", err)
		return nil, err
	}
	clog.Info("install the release success: %s", relaese.Name)
	return relaese, nil
}

// helm upgrade
func (h *Helm) Upgrade(namespace, name, pkgName string, envs map[string]interface{}) (*release.Release, error) {
	actionConfig, err := h.GetActionConfig(namespace)
	if err != nil {
		return nil, err
	}
	file := ""
	if home := homedir.HomeDir(); home != "" {
		file = filepath.Join(home, "helmchartpkg", pkgName)
	} else {
		currentPath, _ := os.Getwd()
		file = filepath.Join(currentPath, "helmchartpkg", pkgName)
	}
	c, err := loader.LoadFile(file)
	if err != nil {
		return nil, err
	}
	upgradeAction := action.NewUpgrade(actionConfig)
	upgradeAction.Namespace = namespace
	relaese, err := upgradeAction.Run(name, c, envs)
	e, _ := json.Marshal(envs)
	clog.Info("%v", string(e))
	if err != nil {
		clog.Info("can not upgrade the release, %v", err)
		return nil, err
	}
	clog.Info("upgrade the release success: %v", relaese.Name)
	return relaese, err
}

// helm uninstall
func (h *Helm) Uninstall(namespace string, name string) error {
	actionConfig, err := h.GetActionConfig(namespace)
	if err != nil {
		return err
	}
	uninstallAction := action.NewUninstall(actionConfig)
	relaese, err := uninstallAction.Run(name)
	if err != nil {
		clog.Info("can not uninstall the release, %v", err)
		return err
	}
	clog.Info("uninstall the release success: %v", relaese.Info)
	return nil
}
