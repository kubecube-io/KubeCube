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

	"github.com/kubecube-io/kubecube/pkg/clients/kubernetes"
	"github.com/kubecube-io/kubecube/pkg/scout"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/kubeconfig"

	"github.com/kubecube-io/kubecube/pkg/apis"
	clusterv1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	toolcache "k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

const (
	add = iota
	update
	del
)

// StartMultiClusterSync as a backend to sync cluster info to memory.
// Closed when as a leader.
func StartMultiClusterSync(ctx context.Context) {
	config, err := ctrl.GetConfig()
	if err != nil {
		panic(err)
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(apis.AddToScheme(scheme))

	c, err := cache.New(config, cache.Options{Scheme: scheme})
	if err != nil {
		panic(err)
	}

	cluster := clusterv1.Cluster{}
	informer, err := c.GetInformer(ctx, &cluster)
	if err != nil {
		panic(err)
	}

	informer.AddEventHandler(toolcache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			doSync(add, obj)
		},
		DeleteFunc: func(obj interface{}) {
			doSync(del, obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			doSync(update, newObj)
		},
	})

	err = c.Start(ctx)
	if err != nil {
		panic(err)
	}
}

// doSync do real sync action, sync action must be not affect controller logic
func doSync(action int, obj interface{}) {
	if obj == nil {
		return
	}

	cluster, ok := obj.(*clusterv1.Cluster)
	if !ok {
		clog.Warn("sync object is not type cluster, got: %v", cluster)
		return
	}

	switch action {
	case add:
		skip, err := addInternalCluster(*cluster)
		if err != nil {
			clog.Error("skip add internal cluster %v failed: %v", cluster.Name, err)
		}
		if !skip || cluster.Name == constants.PivotCluster {
			// start to scout for warden
			err = MultiClusterMgr.ScoutFor(context.Background(), cluster.Name)
			if err != nil {
				clog.Error("scout for %v warden failed: %v", cluster.Name, err)
			}
		}
	case update:
		// temporarily not support update
	case del:
		err := MultiClusterMgr.Del(cluster.Name)
		if err != nil {
			clog.Error(err.Error())
		}
	default:
		clog.Warn("unknown action when sync cluster")
	}
}

// addInternalCluster build internal cluster of cluster cr and add it
// todo(weilaaa): to optimize
func addInternalCluster(cluster clusterv1.Cluster) (bool, error) {
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

		c := new(InternalCluster)
		c.StopCh = make(chan struct{})
		c.Config = config
		c.Client = kubernetes.NewClientFor(config, c.StopCh)
		c.Scout = scout.NewScout(cluster.Name, 0, 0, pivotCluster.Client.Direct(), c.StopCh)

		err = MultiClusterMgr.Add(cluster.Name, c)
		if err != nil {
			return true, fmt.Errorf("add internal cluster failed: %v", err)
		}
	}

	return false, nil
}
