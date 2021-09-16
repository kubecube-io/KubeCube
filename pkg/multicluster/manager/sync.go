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

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	toolcache "k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/kubecube-io/kubecube/pkg/apis"
	clusterv1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
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
			// we only care about delete action
			oldCluster := oldObj.(*clusterv1.Cluster)
			newCluster := newObj.(*clusterv1.Cluster)
			initFailedState, ProcessingState := clusterv1.ClusterInitFailed, clusterv1.ClusterProcessing
			if oldCluster.Status.State == &initFailedState &&
				newCluster.Status.State == &ProcessingState {
				doSync(update, newObj)
			}
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

	clog.Info("cluster sync reconcile cluster %v, action: %v", cluster.Name, action)

	switch action {
	case add, update:
		skip, err := AddInternalCluster(*cluster)
		if err != nil {
			clog.Error("add internal cluster %v failed: %v", cluster.Name, err)
			return
		}
		if !skip || cluster.Name == constants.PivotCluster {
			// start to scout for warden
			err = MultiClusterMgr.ScoutFor(context.Background(), cluster.Name)
			if err != nil {
				clog.Error("scout for %v warden failed: %v", cluster.Name, err)
			}
		}
	case del:
		err := MultiClusterMgr.Del(cluster.Name)
		if err != nil {
			clog.Warn(err.Error())
		}
	default:
		clog.Warn("unknown action when sync cluster")
	}
}
