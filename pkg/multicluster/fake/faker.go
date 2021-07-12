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

package fake

import (
	"context"
	"fmt"
	"sync"

	"github.com/kubecube-io/kubecube/pkg/multicluster/manager"

	"k8s.io/client-go/rest"

	"github.com/kubecube-io/kubecube/pkg/clients/kubernetes/fake"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/kubecube-io/kubecube/pkg/clients/kubernetes"
)

var _ manager.MultiClustersManager = &FakerMultiClusterMgr{}

// FakerMultiClusterMgr implements multicluster.MultiClustersManager
type FakerMultiClusterMgr struct {
	sync.RWMutex
	Clusters map[string]*manager.InternalCluster
}

var (
	IsFake bool

	FakeMultiClusterMgr manager.MultiClustersManager
)

// InitFakeMultiClusterMgrWithOpts must be called at first in testing
func InitFakeMultiClusterMgrWithOpts(opts *fake.Options) {
	m := &FakerMultiClusterMgr{Clusters: make(map[string]*manager.InternalCluster)}

	c := new(manager.InternalCluster)
	c.Client = fake.NewFakeClients(opts)
	c.Config = &rest.Config{Host: "127.0.0.1", ContentConfig: rest.ContentConfig{GroupVersion: &v1.SchemeGroupVersion, NegotiatedSerializer: scheme.Codecs}}

	err := m.Add(constants.PivotCluster, c)
	if err != nil {
		clog.Fatal("init multi cluster mgr failed: %v", err)
	}

	IsFake = true
	FakeMultiClusterMgr = m
}

func (m *FakerMultiClusterMgr) Add(cluster string, c *manager.InternalCluster) error {
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

func (m *FakerMultiClusterMgr) Get(cluster string) (*manager.InternalCluster, error) {
	m.RLock()
	defer m.RUnlock()

	c, ok := m.Clusters[cluster]
	if !ok {
		return nil, fmt.Errorf("get: internal cluster %s not found", cluster)
	}

	return c, nil
}

func (m *FakerMultiClusterMgr) Del(cluster string) error {
	m.Lock()
	defer m.Unlock()

	_, ok := m.Clusters[cluster]
	if !ok {
		return fmt.Errorf("delete: internal cluster %s not found", cluster)
	}

	delete(m.Clusters, cluster)

	return nil
}

func (m *FakerMultiClusterMgr) FuzzyCopy() map[string]*manager.FuzzyCluster {
	m.RLock()
	defer m.RUnlock()

	clusters := make(map[string]*manager.FuzzyCluster)
	for name, v := range m.Clusters {
		clusters[name] = &manager.FuzzyCluster{
			Name:   name,
			Client: v.Client,
		}
	}

	return clusters
}

// ScoutFor do nothing in testing
func (m *FakerMultiClusterMgr) ScoutFor(ctx context.Context, cluster string) error {
	return nil
}

func (m *FakerMultiClusterMgr) GetClient(cluster string) (kubernetes.Client, error) {
	c, err := m.Get(cluster)
	if err != nil {
		return nil, err
	}

	return c.Client, err
}

func Interface() manager.MultiClustersManager {
	return FakeMultiClusterMgr
}
