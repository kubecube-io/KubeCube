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
	"github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"k8s.io/apimachinery/pkg/version"
)

// Manager access to internal cluster
type Manager interface {
	// Add runtime cache in memory
	Add(cluster string, internalCluster *InternalCluster) error
	Get(cluster string) (*InternalCluster, error)
	Del(cluster string) error

	// Version the k8s version about cluster
	Version(cluster string) (*version.Info, error)

	// FuzzyCopy return fuzzy cluster of raw
	FuzzyCopy() map[string]*FuzzyCluster

	// ScoutFor scout heartbeat for warden
	ScoutFor(ctx context.Context, cluster string) error

	// GetClient get client for cluster by name
	GetClient(cluster string) (client.Client, error)

	// ListClustersByType list clusters by given type
	ListClustersByType(t clusterType) []*InternalCluster

	// PivotCluster get pivot cluster
	PivotCluster() *InternalCluster
}

// Interface the way to be used outside for multi cluster manager
func Interface() Manager {
	if isFake {
		return fakeMultiClusterMgr
	}
	return ManagerImpl
}
