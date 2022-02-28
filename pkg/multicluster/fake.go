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
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/version"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"github.com/kubecube-io/kubecube/pkg/multicluster/client/fake"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
)

var _ Manager = &FakerManagerImpl{}

// FakerManagerImpl implements Manager
type FakerManagerImpl struct {
	sync.RWMutex
	Clusters map[string]*InternalCluster
}

var (
	isFake bool

	fakeMultiClusterMgr Manager
)

// InitFakeMultiClusterMgrWithOpts must be called at first in testing
func InitFakeMultiClusterMgrWithOpts(opts *fake.Options) {
	m := &FakerManagerImpl{Clusters: make(map[string]*InternalCluster)}

	c := new(InternalCluster)
	c.Client = fake.NewFakeClients(opts)
	c.Config = &rest.Config{Host: "127.0.0.1", ContentConfig: rest.ContentConfig{GroupVersion: &v1.SchemeGroupVersion, NegotiatedSerializer: scheme.Codecs}}

	err := m.Add(constants.LocalCluster, c)
	if err != nil {
		clog.Fatal("init multi cluster mgr failed: %v", err)
	}

	isFake = true
	fakeMultiClusterMgr = m
}

func (m *FakerManagerImpl) Add(cluster string, c *InternalCluster) error {
	m.Lock()
	defer m.Unlock()

	if c.Client == nil {
		return fmt.Errorf("add: %s, warden and client should not be nil", cluster)
	}

	_, ok := m.Clusters[cluster]
	if ok {
		return fmt.Errorf("add: internal cluster %s aready exist", cluster)
	}

	m.Clusters[cluster] = c

	return nil
}

func (m *FakerManagerImpl) Get(cluster string) (*InternalCluster, error) {
	m.RLock()
	defer m.RUnlock()

	c, ok := m.Clusters[cluster]
	if !ok {
		return nil, fmt.Errorf("get: internal cluster %s not found", cluster)
	}

	return c, nil
}

func (m *FakerManagerImpl) Del(cluster string) error {
	m.Lock()
	defer m.Unlock()

	_, ok := m.Clusters[cluster]
	if !ok {
		return fmt.Errorf("delete: internal cluster %s not found", cluster)
	}

	delete(m.Clusters, cluster)

	return nil
}

func (m *FakerManagerImpl) FuzzyCopy() map[string]*FuzzyCluster {
	m.RLock()
	defer m.RUnlock()

	clusters := make(map[string]*FuzzyCluster)
	for name, v := range m.Clusters {
		clusters[name] = &FuzzyCluster{
			Name:   name,
			Client: v.Client,
		}
	}

	return clusters
}

// ScoutFor do nothing in testing
func (m *FakerManagerImpl) ScoutFor(ctx context.Context, cluster string) error {
	return nil
}

func (m *FakerManagerImpl) GetClient(cluster string) (client.Client, error) {
	c, err := m.Get(cluster)
	if err != nil {
		return nil, err
	}

	return c.Client, err
}

func (m *FakerManagerImpl) ListClustersByType(t clusterType) []*InternalCluster {
	m.RLock()
	defer m.RUnlock()

	var clusters []*InternalCluster
	for _, v := range m.Clusters {
		if v.Type == t {
			clusters = append(clusters, v)
		}
	}

	return clusters
}

func (m *FakerManagerImpl) PivotCluster() *InternalCluster {
	clusters := m.ListClustersByType(PivotCluster)
	if len(clusters) > 0 {
		return clusters[0]
	}
	return nil
}

func (m *FakerManagerImpl) Version(cluster string) (*version.Info, error) {
	return nil, nil
}
