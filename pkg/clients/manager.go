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

package clients

import (
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	"github.com/kubecube-io/kubecube/pkg/multicluster/client"
)

// Clients aggregates all clients of cube needed
type Clients interface {
	Kubernetes(cluster string) client.Client
}

// genericClientSet is global cube cube client that must init at first
var genericClientSet = &cubeClientSet{}

type cubeClientSet struct {
	k8s multicluster.Manager
}

// InitCubeClientSetWithOpts initialize global clients with given config.
func InitCubeClientSetWithOpts(opts *Config) {
	genericClientSet.k8s = multicluster.Interface()
}

// Interface the entry for cube client
func Interface() Clients {
	return genericClientSet
}

// Kubernetes get the indicate client for cluster, we log error instead of return it
// for convenience, caller needs to determine whether the return value is nil
func (c *cubeClientSet) Kubernetes(cluster string) client.Client {
	cli, err := c.k8s.GetClient(cluster)
	if err != nil {
		clog.Warn(err.Error())
		return nil
	}

	return cli
}
