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

package manager

import "github.com/kubecube-io/kubecube/pkg/clients/kubernetes"

func (m *MultiClustersMgr) GetClient(cluster string) (kubernetes.Client, error) {
	c, err := m.Get(cluster)
	if err != nil {
		return nil, err
	}

	return c.Client, err
}
