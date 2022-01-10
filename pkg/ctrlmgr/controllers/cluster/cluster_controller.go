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

package controllers

import (
	"context"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	clusterv1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	"github.com/kubecube-io/kubecube/pkg/utils"
	"github.com/kubecube-io/kubecube/pkg/utils/kubeconfig"
)

var (
	log clog.CubeLogger

	_ reconcile.Reconciler = &ClusterReconciler{}
)

const clusterFinalizer = "cluster.finalizers.kubecube.io"

// ClusterReconciler deploy warden to member cluster
// when create event trigger
type ClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// todo: remove this field in the future
	pivotCluster *clusterv1.Cluster

	// retryQueue holds all retrying cluster that has the way to stop retrying
	retryQueue sync.Map

	// Affected is a channel of event.GenericEvent (see "Watching Channels" in
	// https://book-v1.book.kubebuilder.io/beyond_basics/controller_watches.html) that is used to
	// enqueue additional objects that need updating.
	Affected chan event.GenericEvent
}

func newReconciler(mgr manager.Manager) (*ClusterReconciler, error) {
	log = clog.WithName("cluster")

	r := &ClusterReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Affected: make(chan event.GenericEvent),
	}
	return r, nil
}

//+kubebuilder:rbac:groups=cluster.kubecube.io,resources=clusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.kubecube.io,resources=clusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cluster.kubecube.io,resources=clusters/finalizers,verbs=update

func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log.Info("Reconcile cluster %v", req.Name)

	cluster := clusterv1.Cluster{}

	// get cr ensure current cluster cr exist
	if err := r.Get(ctx, req.NamespacedName, &cluster); err != nil {
		if errors.IsNotFound(err) {
			log.Debug("current cluster %v has deleted, skip", cluster.Name)
			return ctrl.Result{}, nil
		}
		log.Error("get current cluster %v cr failed: %v", cluster.Name, err)
		return ctrl.Result{}, err
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if cluster.ObjectMeta.DeletionTimestamp == nil {
		// ensure finalizer
		if err := r.ensureFinalizer(ctx, &cluster); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		// relation remove
		if err := r.removeFinalizer(ctx, &cluster); err != nil {
			return ctrl.Result{}, err
		}
		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	return r.syncCluster(ctx, cluster)
}

func (r *ClusterReconciler) syncCluster(ctx context.Context, cluster clusterv1.Cluster) (ctrl.Result, error) {
	// update cluster status to processing
	err := utils.UpdateClusterStatusByState(ctx, r.Client, &cluster, clusterv1.ClusterProcessing)
	if err != nil {
		return ctrl.Result{}, err
	}
	log.Info("Cluster %v is processing", cluster.Name)

	// try connect to cluster, tempClient will be GC after function down
	tempClient, err := tryConnectCluster(cluster)
	if err != nil {
		// todo: what if kubeconfig is wrong
		log.Error(err.Error())
		_ = utils.UpdateClusterStatusByState(ctx, r.Client, &cluster, clusterv1.ClusterInitFailed)
		r.enqueue(cluster)
		return ctrl.Result{}, nil
	}
	log.Info("Handshake with cluster %v success", cluster.Name)

	// deploy resources to cluster
	err = deployResources(ctx, tempClient, &cluster, r.pivotCluster)
	if err != nil {
		log.Error("deploy resource failed: %v", err)
		_ = utils.UpdateClusterStatusByState(ctx, r.Client, &cluster, clusterv1.ClusterInitFailed)
		return ctrl.Result{}, err
	}
	log.Info("Ensure resources in cluster %v success", cluster.Name)

	// generate internal cluster for current cluster and add
	// it to the cache of multi cluster manager
	err = multicluster.AddInternalClusterWithScout(cluster)
	if err != nil {
		log.Error(err.Error())
		_ = utils.UpdateClusterStatusByState(ctx, r.Client, &cluster, clusterv1.ClusterInitFailed)
		r.enqueue(cluster)
		return ctrl.Result{}, nil
	}
	log.Info("Ensure cluster %v in internal clusters success", cluster.Name)

	// start to scout loop for memberCluster warden, non-block
	// status convert to normal after receive birth cry from scout
	err = multicluster.Interface().ScoutFor(context.Background(), cluster.Name)
	if err != nil {
		log.Error("start scout for cluster %v failed", cluster.Name)
		_ = utils.UpdateClusterStatusByState(ctx, r.Client, &cluster, clusterv1.ClusterInitFailed)
		return ctrl.Result{}, err
	}

	// no need lock cause write by one at same time
	if !cluster.Spec.IsMemberCluster {
		r.pivotCluster = &cluster
	}

	return ctrl.Result{}, nil
}

// It enqueues a cluster for later reconciliation. This occurs in a goroutine
// so the caller doesn't block; since the reconciler is never garbage-collected, this is safe.
func (r *ClusterReconciler) enqueue(cluster clusterv1.Cluster) {
	const (
		// todo(weilaaa): add args for those
		retryInterval = 7 * time.Second
		retryTimeout  = 12 * time.Hour
	)

	config, _ := kubeconfig.LoadKubeConfigFromBytes(cluster.Spec.KubeConfig)

	// set retry timeout is 12 hours
	ctx, cancel := context.WithTimeout(context.Background(), retryTimeout)

	_, loaded := r.retryQueue.LoadOrStore(cluster.Name, cancel)
	if loaded {
		// directly return if this cluster was already in retry queue
		return
	}

	// try to reconnect with cluster api server, requeue if every is ok
	go func() {
		log.Info("cluster %v init failed, keep retry background", cluster.Name)

		// pop from retry queue when reconnected or context exceed or context canceled
		defer r.retryQueue.Delete(cluster.Name)

		for {
			select {
			case <-time.Tick(retryInterval):
				_, err := client.New(config, client.Options{Scheme: r.Scheme})
				if err == nil {
					log.Info("enqueuing cluster %v for reconciliation", cluster.Name)
					r.Affected <- event.GenericEvent{Object: &cluster}
					return
				}
			case <-ctx.Done():
				log.Info("cluster %v retry task stopped: %v", cluster.Name, ctx.Err())

				// retrying timeout need update status
				// todo(weilaaa): to allow user reconnect cluster manually
				if ctx.Err().Error() == "context deadline exceeded" {
					_ = utils.UpdateClusterStatusByState(ctx, r.Client, &cluster, clusterv1.ClusterReconnectedFailed)
				}

				return
			}
		}
	}()
}

// SetupWithManager sets up the controller with the Manager.
func SetupWithManager(mgr ctrl.Manager) error {
	r, err := newReconciler(mgr)
	if err != nil {
		return err
	}

	// filter update event
	predicateFunc := predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			if !updateEvent.ObjectNew.GetDeletionTimestamp().IsZero() {
				return true
			}
			return false
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			// we use generic event to process init failed cluster
			return true
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1.Cluster{}).
		Watches(&source.Channel{Source: r.Affected}, &handler.EnqueueRequestForObject{}).
		WithEventFilter(predicateFunc).
		Complete(r)
}
