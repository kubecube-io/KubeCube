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
	"net/http"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"

	clusterv1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"github.com/kubecube-io/kubecube/pkg/multicluster/scout"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/exit"
	"github.com/kubecube-io/kubecube/pkg/utils/kubeconfig"
)

// clusterType indicates the internal cluster type
type clusterType int

const (
	LocalCluster clusterType = iota
	PivotCluster
	MemberCluster
	AllCluster
)

// ManagerImpl instance implement interface,
// init local cluster at first.
var ManagerImpl = newMultiClusterMgr()

// newMultiClusterMgr init MultiClustersMgr with local cluster.
// local cluster has no raw config neither scout.
func newMultiClusterMgr() *MultiClustersMgr {
	m := &MultiClustersMgr{Clusters: make(map[string]*InternalCluster)}

	// init local cluster at first
	config, err := ctrl.GetConfig()
	if err != nil {
		clog.Warn("get kubeconfig failed: %v, only allowed when testing", err)
		return nil
	}
	ts, err := rest.TransportFor(config)
	if err != nil {
		clog.Warn("load RoundTripper failed: %v", err)
		return nil
	}

	c := new(InternalCluster)
	c.StopCh = make(chan struct{})
	c.Config = config
	c.transport = ts
	c.Type = LocalCluster
	c.Client, err = client.NewClientFor(exit.SetupCtxWithStop(context.Background(), c.StopCh), config)
	if err != nil {
		// early exit when connect to the k8s apiserver of control plane failed
		clog.Fatal("make client for local cluster failed: %v", err)
	}
	c.Version, err = c.Client.Discovery().ServerVersion()
	if err != nil {
		clog.Fatal("discovery cluster version failed: %v", err)
	}

	err = m.Add(constants.LocalCluster, c)
	if err != nil {
		clog.Fatal("init multi cluster mgr failed: %v", err)
	}

	return m
}

// InternalCluster represent a cluster runtime contains
// client and internal warden.
type InternalCluster struct {
	// Name the cluster name
	Name string

	// Type of cluster
	Type clusterType

	// Client holds all the clients needed
	Client client.Client

	// Scout knows the health status of cluster and keep watch
	Scout *scout.Scout

	// Config bind to a real cluster
	Config *rest.Config

	// Transport bind to a real cluster round tripper
	transport http.RoundTripper

	// Version the k8s Version about internal cluster
	Version *version.Info

	// RawCluster holds raw cluster
	RawCluster *clusterv1.Cluster

	// StopCh for closing channel when delete cluster, goroutine
	// of cache and scout will exit gracefully.
	StopCh chan struct{}
}

func NewInternalCluster(cluster clusterv1.Cluster) (*InternalCluster, error) {
	config, err := kubeconfig.LoadKubeConfigFromBytes(cluster.Spec.KubeConfig)
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig failed: %v", err)
	}
	ts, err := rest.TransportFor(config)
	if err != nil {
		return nil, fmt.Errorf("load RoundTripper failed: %v", err)
	}
	// allocate mem address to avoid nil
	cluster.Status.State = new(clusterv1.ClusterState)

	var clusterType clusterType
	if cluster.Spec.IsMemberCluster {
		clusterType = MemberCluster
	} else {
		clusterType = PivotCluster
	}

	c := new(InternalCluster)
	c.Name = cluster.Name
	c.StopCh = make(chan struct{})
	c.Config = config
	c.transport = ts
	c.Type = clusterType
	c.RawCluster = cluster.DeepCopy()
	c.Client, err = client.NewClientFor(exit.SetupCtxWithStop(context.Background(), c.StopCh), config)
	if err != nil {
		return nil, err
	}
	c.Version, err = c.Client.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}

	return c, nil
}

// MultiClustersMgr a memory cache for runtime cluster.
type MultiClustersMgr struct {
	sync.RWMutex
	Clusters map[string]*InternalCluster
}

func (m *MultiClustersMgr) Add(cluster string, c *InternalCluster) error {
	m.Lock()
	defer m.Unlock()

	_, ok := m.Clusters[cluster]
	if ok {
		return fmt.Errorf("add: internal cluster %s aready exist", cluster)
	}

	m.Clusters[cluster] = c

	clog.Info("add cluster %v into multi cluster manager", cluster)

	return nil
}

func (m *MultiClustersMgr) Get(cluster string) (*InternalCluster, error) {
	m.RLock()
	defer m.RUnlock()

	c, ok := m.Clusters[cluster]
	if !ok {
		return nil, fmt.Errorf("get: internal cluster %s not found", cluster)
	}

	// ignore cluster health if there is no scout
	if c.Scout == nil {
		return c, nil
	}

	if c.Scout.ClusterHealth() == clusterv1.ClusterAbnormal {
		return c, fmt.Errorf("internal cluster %v is abnormal, wait for recover", cluster)
	}

	return c, nil
}

func (m *MultiClustersMgr) Del(cluster string) error {
	m.Lock()
	defer m.Unlock()

	internalCluster, ok := m.Clusters[cluster]
	if !ok {
		return fmt.Errorf("delete: internal cluster %s not found", cluster)
	}

	// stop goroutines inside internal cluster
	close(internalCluster.StopCh)

	delete(m.Clusters, cluster)

	return nil
}

func (m *MultiClustersMgr) GetClient(cluster string) (client.Client, error) {
	c, err := m.Get(cluster)
	if err != nil {
		return nil, err
	}

	return c.Client, err
}

func (m *MultiClustersMgr) GetTransport(cluster string) (http.RoundTripper, error) {
	c, err := m.Get(cluster)
	if err != nil {
		return nil, err
	}
	return c.transport, err
}

func (m *MultiClustersMgr) Version(cluster string) (*version.Info, error) {
	c, err := m.Get(cluster)
	if err != nil {
		return nil, err
	}

	return c.Version, nil
}

// ScoutFor starts watch for warden intelligence
func (m *MultiClustersMgr) ScoutFor(ctx context.Context, cluster string) error {
	c, err := m.Get(cluster)
	if err != nil {
		return err
	}

	// internal cluster without scout will do nothing
	if c.Scout == nil {
		return nil
	}

	c.Scout.Once.Do(func() {
		clog.Info("Start scout for cluster %v", c.Scout.Cluster)

		ctx = exit.SetupCtxWithStop(ctx, c.Scout.StopCh)

		time.AfterFunc(time.Duration(c.Scout.InitialDelaySeconds)*time.Second, func() {
			go c.Scout.Collect(ctx)
		})
	})

	return nil
}

// ListClustersByType get clusters by given type
// return nil if found no clusters with type.
func (m *MultiClustersMgr) ListClustersByType(t clusterType) []*InternalCluster {
	m.RLock()
	defer m.RUnlock()

	var clusters []*InternalCluster
	for _, v := range m.Clusters {
		if v.Type == t || (t == AllCluster && v.Type != LocalCluster) {
			clusters = append(clusters, v)
		}
	}

	return clusters
}

// ListClustersNameByType get cluster names by given type
// return nil if found no clusters with type.
func (m *MultiClustersMgr) ListClustersNameByType(t clusterType) []string {
	m.RLock()
	defer m.RUnlock()

	var clusterNames []string
	for _, v := range m.Clusters {
		if v.Type == t || (t == AllCluster && v.Type != LocalCluster) {
			clusterNames = append(clusterNames, v.Name)
		}
	}

	return clusterNames
}

// FuzzyCluster be exported for test
type FuzzyCluster struct {
	Name       string
	Config     *rest.Config
	Client     client.Client
	RawCluster *clusterv1.Cluster
}

// FuzzyCopy copy all internal clusters when runtime except local cluster
func (m *MultiClustersMgr) FuzzyCopy() map[string]*FuzzyCluster {
	m.RLock()
	defer m.RUnlock()

	clusters := make(map[string]*FuzzyCluster)
	for name, v := range m.Clusters {
		if name == constants.LocalCluster {
			continue
		}
		// we must new *rest.Config just like deep copy
		cfg, _ := kubeconfig.LoadKubeConfigFromBytes(v.RawCluster.Spec.KubeConfig)
		clusters[name] = &FuzzyCluster{
			Name:       name,
			Config:     cfg,
			Client:     v.Client,
			RawCluster: v.RawCluster,
		}
	}

	return clusters
}

// AddInternalClusterWithScout build internal cluster of cluster and add it
// to multi cluster manager with scout by default opts.
func AddInternalClusterWithScout(cluster clusterv1.Cluster) error {
	return addInternalCluster(cluster, true, 0, 0)
}

// AddInternalClusterWithScoutOpts build internal cluster of cluster and add it
// to multi cluster manager with scout opts.
func AddInternalClusterWithScoutOpts(cluster clusterv1.Cluster, scoutInitialDelaySeconds, scoutWaitTimeoutSeconds int) error {
	return addInternalCluster(cluster, true, scoutInitialDelaySeconds, scoutWaitTimeoutSeconds)
}

// AddInternalCluster build internal cluster of cluster and add it
// to multi cluster manager without scout.
func AddInternalCluster(cluster clusterv1.Cluster) error {
	return addInternalCluster(cluster, false, 0, 0)
}

func addInternalCluster(cluster clusterv1.Cluster, withScout bool, scoutInitialDelaySeconds, scoutWaitTimeoutSeconds int) error {
	_, err := ManagerImpl.Get(cluster.Name)
	if err == nil {
		// return Immediately if active internal cluster exist
		return nil
	}

	c, err := NewInternalCluster(cluster)
	if err != nil {
		return err
	}

	if withScout {
		localCluster, err := ManagerImpl.Get(constants.LocalCluster)
		if err != nil {
			return err
		}
		c.Scout = scout.NewScout(cluster.Name, scoutInitialDelaySeconds, scoutWaitTimeoutSeconds, localCluster.Client.Direct(), c.StopCh)
	}

	err = ManagerImpl.Add(cluster.Name, c)
	if err != nil {
		clog.Warn("add internal cluster failed: %v", err)
	}

	return nil
}
