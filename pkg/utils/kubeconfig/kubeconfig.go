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

package kubeconfig

import (
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// LoadKubeConfigFromBytes load kubeConfig form raw config
func LoadKubeConfigFromBytes(kubeConfig []byte) (*rest.Config, error) {
	clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeConfig)
	if err != nil {
		return nil, err
	}

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	return config, nil
}

// BuildKubeConfigFromRestConfig build kubeConfig from config use cert way
func BuildKubeConfigFromRestConfig(config *rest.Config) ([]byte, error) {
	apiConfig := api.NewConfig()

	apiCluster := &api.Cluster{
		Server:                   config.Host,
		CertificateAuthorityData: config.CAData,
	}

	apiConfig.Clusters["kubernetes"] = apiCluster

	apiConfig.AuthInfos["kubernetes-admin"] = &api.AuthInfo{
		ClientCertificateData: config.CertData,
		ClientKeyData:         config.KeyData,
		Token:                 config.BearerToken,
		TokenFile:             config.BearerTokenFile,
		Username:              config.Username,
		Password:              config.Password,
	}

	apiConfig.Contexts["kubernetes-admin@kubernetes"] = &api.Context{
		Cluster:  "kubernetes",
		AuthInfo: "kubernetes-admin",
	}

	apiConfig.CurrentContext = "kubernetes-admin@kubernetes"

	return clientcmd.Write(*apiConfig)
}

// ConfigMeta is meta of kubeConfig
type ConfigMeta struct {
	Config  *rest.Config
	Cluster string
	User    string
	Token   string
}

// BuildKubeConfigForUser build kubeConfig for specified user, aggregate
// if there are clusters more than one. Only token way support.
func BuildKubeConfigForUser(cms []*ConfigMeta) ([]byte, error) {
	apiConfig := api.NewConfig()

	for _, cm := range cms {

		cluster := cm.Cluster
		authInfo := cluster + "-" + cm.User
		context := authInfo + "@" + cluster

		apiConfig.Clusters[cluster] = &api.Cluster{
			Server:                   cm.Config.Host,
			CertificateAuthorityData: cm.Config.CAData,
		}

		apiConfig.AuthInfos[authInfo] = &api.AuthInfo{
			Token: cm.Token,
		}

		apiConfig.Contexts[context] = &api.Context{
			Cluster:  cluster,
			AuthInfo: authInfo,
		}

		apiConfig.CurrentContext = context

	}

	return clientcmd.Write(*apiConfig)
}
