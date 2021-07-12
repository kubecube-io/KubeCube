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

import (
	"context"
	"fmt"
	"sync"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	hnc "sigs.k8s.io/multi-tenancy/incubator/hnc/api/v1alpha2"

	"github.com/kubecube-io/kubecube/pkg/apis"
	v1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
	"github.com/kubecube-io/kubecube/pkg/utils/kubeconfig"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/client-go/rest"

	"github.com/kubecube-io/kubecube/pkg/clog"

	"github.com/kubecube-io/kubecube/pkg/utils/constants"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/kubecube-io/kubecube/pkg/clients/kubernetes"
	"github.com/kubecube-io/kubecube/pkg/scout"
)

// MultiClustersManager access to internal cluster
type MultiClustersManager interface {
	// Add runtime cache in memory
	Add(cluster string, internalCluster *InternalCluster) error
	Get(cluster string) (*InternalCluster, error)
	Del(cluster string) error

	// FuzzyCopy return fuzzy cluster of raw
	FuzzyCopy() map[string]*FuzzyCluster

	// ScoutFor scout heartbeat for warden
	ScoutFor(ctx context.Context, cluster string) error

	// GetClient get client for cluster
	GetClient(cluster string) (kubernetes.Client, error)
}

// MultiClusterMgr instance implement interface,
// init pivot cluster at first.
var MultiClusterMgr = newMultiClusterMgr()

// newMultiClusterMgr init MultiClustersMgr with pivot internal cluster
func newMultiClusterMgr() *MultiClustersMgr {
	m := &MultiClustersMgr{Clusters: make(map[string]*InternalCluster)}
	config, err := ctrl.GetConfig()
	if err != nil {
		clog.Warn("get kubeconfig failed: %v, only allowed when testing", err)
		return nil
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(apis.AddToScheme(scheme))
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(hnc.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))

	cli, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		clog.Warn(err.Error())
		return nil
	}

	cluster := v1.Cluster{}
	err = cli.Get(context.Background(), types.NamespacedName{Name: constants.PivotCluster}, &cluster)
	if err != nil {
		clog.Warn(err.Error())
		return nil
	}

	cfg, err := kubeconfig.LoadKubeConfigFromBytes(cluster.Spec.KubeConfig)
	if err != nil {
		clog.Fatal(err.Error())
	}

	c := new(InternalCluster)
	c.StopCh = make(chan struct{})
	c.Config = cfg
	c.Client = kubernetes.NewClientFor(cfg, c.StopCh)
	c.Scout = scout.NewScout(constants.PivotCluster, 0, 0, c.Client.Direct(), c.StopCh)

	err = m.Add(constants.PivotCluster, c)
	if err != nil {
		clog.Fatal("init multi cluster mgr failed: %v", err)
	}
	return m
}

// InternalCluster represent a cluster runtime contains
// client and internal warden.
type InternalCluster struct {
	Client kubernetes.Client
	Scout  *scout.Scout

	// Config bind to a real cluster
	Config *rest.Config

	// close channel when delete cluster, goroutine
	// of informer and scout will exit gracefully.
	StopCh chan struct{}
}

// MultiClustersMgr a memory cache for runtime cluster.
type MultiClustersMgr struct {
	sync.RWMutex
	Clusters map[string]*InternalCluster
}

func (m *MultiClustersMgr) Add(cluster string, c *InternalCluster) error {
	m.Lock()
	defer m.Unlock()

	if c.Scout == nil || c.Client == nil {
		return fmt.Errorf("add: %s, warden and client should not be nil", cluster)
	}

	_, ok := m.Clusters[cluster]
	if ok {
		return fmt.Errorf("add: internal cluster %s aready exist", cluster)
	}

	m.Clusters[cluster] = c

	return nil
}

func (m *MultiClustersMgr) Get(cluster string) (*InternalCluster, error) {
	m.RLock()
	defer m.RUnlock()

	c, ok := m.Clusters[cluster]
	if !ok {
		return nil, fmt.Errorf("get: internal cluster %s not found", cluster)
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

// FuzzyCluster be exported for test
type FuzzyCluster struct {
	Name   string
	Config *rest.Config
	Client kubernetes.Client
}

func (m *MultiClustersMgr) FuzzyCopy() map[string]*FuzzyCluster {
	m.RLock()
	defer m.RUnlock()

	clusters := make(map[string]*FuzzyCluster)
	for name, v := range m.Clusters {
		clusters[name] = &FuzzyCluster{
			Name:   name,
			Config: v.Config,
			Client: v.Client,
		}
	}

	return clusters
}
