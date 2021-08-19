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

	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterv1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
)

var (
	log clog.CubeLogger

	_ reconcile.Reconciler = &ClusterReconciler{}
)

const clusterFinalizer = "cluster.finalizers.kubecube.io"

// ClusterReconciler deploy warden to member cluster
// when create event trigger
type ClusterReconciler struct {
	pivotHandleCount int
	client.Client
	Scheme       *runtime.Scheme
	pivotCluster clusterv1.Cluster
}

func newReconciler(mgr manager.Manager) (reconcile.Reconciler, error) {
	log = clog.WithName("cluster")

	pivotCluster := clusterv1.Cluster{}
	err := clients.Interface().Kubernetes(constants.PivotCluster).Direct().Get(context.Background(), types.NamespacedName{Name: constants.PivotCluster}, &pivotCluster)
	if err != nil {
		return nil, err
	}

	r := &ClusterReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		pivotCluster: pivotCluster,
	}
	return r, nil
}

//+kubebuilder:rbac:groups=cluster.kubecube.io,resources=clusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.kubecube.io,resources=clusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cluster.kubecube.io,resources=clusters/finalizers,verbs=update

// Reconcile of cluster do things below:
// 1. build and add internal cluster.
// 2. issues resources to specified cluster.
// 3. watch healthy condition for warden.
// 4. update cluster status here.
func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	clog.Info("Reconcile cluster %v", req.Name)

	isMemberCluster := !(req.Name == constants.PivotCluster)
	currentCluster := r.pivotCluster

	// pivot cluster only process once
	if !isMemberCluster {
		if r.pivotHandleCount > 0 {
			return ctrl.Result{}, nil
		}
		r.pivotHandleCount++
	}

	if isMemberCluster {
		// get cr ensure memberCluster cr exist
		if err := r.Get(ctx, req.NamespacedName, &currentCluster); err != nil {
			if errors.IsNotFound(err) {
				log.Debug("memberCluster %v has deleted, skip", currentCluster.Name)
				return ctrl.Result{}, nil
			}
			log.Error("get memberCluster %v cr failed: %v", currentCluster.Name, err)
			return ctrl.Result{}, err
		}

		// examine DeletionTimestamp to determine if object is under deletion
		if currentCluster.ObjectMeta.DeletionTimestamp.IsZero() {
			// ensure finalizer
			if err := r.ensureFinalizer(ctx, &currentCluster); err != nil {
				return ctrl.Result{Requeue: true}, err
			}
		} else {
			// relation remove
			if err := r.removeFinalizer(ctx, &currentCluster); err != nil {
				return ctrl.Result{}, err
			}
			// Stop reconciliation as the item is being deleted
			return ctrl.Result{}, nil
		}

		skip, err := addInternalCluster(currentCluster)
		if err != nil {
			clog.Error(err.Error())
		}
		if skip {
			return ctrl.Result{}, err
		}
		clog.Info("add cluster %v to internal clusters success", currentCluster.Name)
	}

	// deploy resources to cluster
	err := deployResources(ctx, currentCluster, r.pivotCluster)
	if err != nil {
		log.Error("deploy resource failed: %v", err)
	}

	// start to scout loop for memberCluster warden, non-block
	err = multicluster.Interface().ScoutFor(context.Background(), currentCluster.Name)
	if err != nil {
		log.Error("start scout for memberCluster %v failed", currentCluster.Name)
	}

	// update cluster status to processing
	err = r.updateClusterStatus(ctx, currentCluster)
	if err != nil {
		log.Error("update cluster %v status failed", currentCluster.Name)
	}

	return ctrl.Result{}, nil
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
			return true
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1.Cluster{}).
		WithEventFilter(predicateFunc).
		Complete(r)
}
