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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	hnc "sigs.k8s.io/multi-tenancy/incubator/hnc/api/v1alpha2"

	"github.com/kubecube-io/kubecube/pkg/apis"
	clusterv1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
	"github.com/kubecube-io/kubecube/pkg/clients/kubernetes"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/scout"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/kubeconfig"
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
		clog.Fatal("connect to pivot cluster failed: %v", err)
	}

	cluster := clusterv1.Cluster{}
	err = cli.Get(context.Background(), types.NamespacedName{Name: constants.PivotCluster}, &cluster)
	if err != nil {
		clog.Fatal("get pivot cluster failed: %v", err)
	}

	cfg, err := kubeconfig.LoadKubeConfigFromBytes(cluster.Spec.KubeConfig)
	if err != nil {
		clog.Fatal("invalid kubeconfig of pivot cluster: %v", err)
	}

	c := new(InternalCluster)
	c.StopCh = make(chan struct{})
	c.Config = cfg
	c.Client, err = kubernetes.NewClientFor(cfg, c.StopCh)
	if err != nil {
		// early exit when connect to the k8s apiserver of control plane failed
		clog.Fatal("make client for pivot cluster failed: %v", err)
	}

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
	// Client holds all the clients needed
	Client kubernetes.Client

	// Scout knows the health status of cluster and keep watch
	Scout *scout.Scout

	// Config bind to a real cluster
	Config *rest.Config

	// StopCh for closing channel when delete cluster, goroutine
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

	if c.Scout == nil {
		return fmt.Errorf("add: %s, scout should not be nil", cluster)
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

	if c.Scout.ClusterState == clusterv1.ClusterAbnormal {
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

// AddInternalCluster build internal cluster of cluster and add it
// to multi cluster manager
func AddInternalCluster(cluster clusterv1.Cluster) (bool, error) {
	_, err := MultiClusterMgr.Get(cluster.Name)
	if err == nil {
		// return Immediately if active internal cluster exist
		return true, nil
	} else {
		// create internal cluster relate with cluster cr
		config, err := kubeconfig.LoadKubeConfigFromBytes(cluster.Spec.KubeConfig)
		if err != nil {
			return true, fmt.Errorf("load kubeconfig failed: %v", err)
		}

		pivotCluster, err := MultiClusterMgr.Get(constants.PivotCluster)
		if err != nil {
			return true, err
		}

		// allocate mem address to avoid nil
		cluster.Status.State = new(clusterv1.ClusterState)

		c := new(InternalCluster)
		c.StopCh = make(chan struct{})
		c.Config = config
		c.Scout = scout.NewScout(cluster.Name, 0, 0, pivotCluster.Client.Direct(), c.StopCh)
		c.Client, err = kubernetes.NewClientFor(config, c.StopCh)
		if err != nil {
			// NewClientFor failed mean cluster init failed that need
			// requeue as soon as reconnect with cluster api-server success
			*cluster.Status.State = clusterv1.ClusterInitFailed
			return false, err
		}

		*cluster.Status.State = clusterv1.ClusterProcessing

		err = MultiClusterMgr.Add(cluster.Name, c)
		if err != nil {
			return true, fmt.Errorf("add internal cluster failed: %v", err)
		}
	}

	return false, nil
}
