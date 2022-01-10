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
package github

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubecube-io/kubecube/pkg/authentication"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/warden/localmgr/controllers/hotplug"
)

const configMapName = "kubecube-auth-config"

func getConfig() authentication.GitHubConfig {
	var gitHubConfig authentication.GitHubConfig

	kClient := clients.Interface().Kubernetes(constants.LocalCluster).Cache()
	if kClient == nil {
		clog.Error("get pivot cluster client is nil")
		return gitHubConfig
	}
	cm := &v1.ConfigMap{}
	err := kClient.Get(context.Background(), client.ObjectKey{Name: configMapName, Namespace: constants.CubeNamespace}, cm)
	if err != nil {
		clog.Error("get configmap from K8s err: %v", err)
		return gitHubConfig
	}

	config := cm.Data["github"]
	if config == "" {
		clog.Error("github config is nil")
		return gitHubConfig
	}
	configJson, error := hotplug.YamlStringToJson(config)
	if error != nil {
		clog.Error("%v", error.Error())
		return gitHubConfig
	}
	if configJson["enabled"] != nil && configJson["enabled"].(bool) == true {
		gitHubConfig.GitHubIsEnable = true
	} else {
		return gitHubConfig
	}

	if configJson["clientId"] != nil {
		gitHubConfig.ClientID = configJson["clientId"].(string)
	}
	if configJson["clientSecret"] != nil {
		gitHubConfig.ClientSecret = configJson["clientSecret"].(string)
	}
	return gitHubConfig
}
