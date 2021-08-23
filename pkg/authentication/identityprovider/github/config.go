package github

import (
	"context"

	"github.com/kubecube-io/kubecube/pkg/authentication"

	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const configMapName = "kubecube-auth-config"

func getConfig() authentication.GitHubConfig {
	var config authentication.GitHubConfig

	kClient := clients.Interface().Kubernetes(constants.PivotCluster).Cache()
	cm := &v1.ConfigMap{}
	err := kClient.Get(context.Background(), client.ObjectKey{Name: configMapName, Namespace: constants.CubeNamespace}, cm)
	if err != nil {
		clog.Error("get configmap from K8s err: %v", err)
		return config
	}

	if cm.Data["github.enabled"] == "true" {
		config.GitHubIsEnable = true
	} else {
		config.GitHubIsEnable = false
	}
	config.ClientID = cm.Data["github.clientId"]
	config.ClientSecret = cm.Data["github.clientSecret"]
	return config
}
