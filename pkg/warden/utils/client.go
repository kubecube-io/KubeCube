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

package utils

import (
	"strings"

	"k8s.io/api/authentication/v1beta1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/kubecube-io/kubecube/pkg/authentication/authenticators/jwt"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
)

// Cluster local cluster name
// deprecated: global v is not good
var Cluster string

func GetPivotConfig(pivotClusterKubeConfig, pivotCubeHost string) (*restclient.Config, error) {
	if len(pivotClusterKubeConfig) != 0 {
		cfg, err := clientcmd.BuildConfigFromFlags("", pivotClusterKubeConfig)
		if err != nil {
			return nil, err
		}
		return cfg, nil
	}
	authJwtImpl := jwt.GetAuthJwtImpl()
	token, errInfo := authJwtImpl.GenerateTokenWithExpired(&v1beta1.UserInfo{Username: "admin"}, 100*365*24*3600)
	if errInfo != nil {
		return nil, errInfo
	}

	host := pivotCubeHost
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		// default, use https as scheme
		host = "https://" + host
	}
	if strings.HasSuffix(host, "/") {
		host = strings.TrimSuffix(host, "/")
	}
	host += constants.ApiK8sProxyPath

	cfg := &restclient.Config{
		Host:        host,
		BearerToken: token,
		TLSClientConfig: restclient.TLSClientConfig{
			Insecure: true,
		},
	}
	return cfg, nil

}
