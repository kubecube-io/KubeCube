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

package multicluster

import (
	"sync"

	"github.com/kubecube-io/kubecube/pkg/conversion"
)

var _ conversion.MultiVersionConverter = &DefaultMultiVersionConverter{}

type DefaultMultiVersionConverter struct {
	rw              sync.RWMutex
	versionConverts map[string]*conversion.VersionConverter
	multiClusterMgr Manager
}

func NewDefaultMultiVersionConverter(m Manager) conversion.MultiVersionConverter {
	return &DefaultMultiVersionConverter{
		versionConverts: map[string]*conversion.VersionConverter{},
		multiClusterMgr: m,
	}
}

func (m *DefaultMultiVersionConverter) GetVersionConvert(cluster string) (*conversion.VersionConverter, error) {
	m.rw.RLock()
	c, find := m.versionConverts[cluster]
	m.rw.RUnlock()
	if find {
		return c, nil
	}

	// new cluster version convert if do not exist
	ic, err := m.multiClusterMgr.Get(cluster)
	if err != nil {
		return nil, err
	}

	newc, err := conversion.NewVersionConvertor(ic.Client.Discovery(), nil)
	if err != nil {
		return nil, err
	}

	m.rw.Lock()
	m.versionConverts[cluster] = newc
	m.rw.Unlock()

	return newc, nil
}
