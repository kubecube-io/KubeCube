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

	"github.com/kubecube-io/kubecube/pkg/utils/strslice"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/kubecube-io/kubecube/pkg/clog"

	"github.com/kubecube-io/kubecube/pkg/multicluster"

	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	Scheme *runtime.Scheme
}

func newReconciler(mgr manager.Manager) (reconcile.Reconciler, error) {
	log = clog.WithName("cluster")

	r := &ClusterReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
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
	var pivotCluster clusterv1.Cluster
	if err := r.Get(ctx, types.NamespacedName{Name: constants.PivotCluster}, &pivotCluster); err != nil {
		log.Error("get pivot cluster %v cr failed: %v", pivotCluster.Name, err)
	}

	var (
		isMemberCluster = !(req.Name == constants.PivotCluster)
		memberCluster   = pivotCluster
	)

	// pivot cluster only process once
	if !isMemberCluster {
		if r.pivotHandleCount > 0 {
			return ctrl.Result{}, nil
		}
		r.pivotHandleCount++
	}

	if isMemberCluster {
		// get cr ensure memberCluster cr exist
		if err := r.Get(ctx, req.NamespacedName, &memberCluster); err != nil {
			if errors.IsNotFound(err) {
				log.Debug("memberCluster %v has deleted, skip", memberCluster.Name)
				return ctrl.Result{}, nil
			}
			log.Error("get memberCluster %v cr failed: %v", memberCluster.Name, err)
		}

		// examine DeletionTimestamp to determine if object is under deletion
		if memberCluster.ObjectMeta.DeletionTimestamp.IsZero() {
			// The object is not being deleted, so if it does not have our finalizer,
			// then lets add the finalizer and update the object. This is equivalent
			// registering our finalizer.
			if !strslice.ContainsString(memberCluster.ObjectMeta.Finalizers, clusterFinalizer) {
				memberCluster.ObjectMeta.Finalizers = append(memberCluster.ObjectMeta.Finalizers, clusterFinalizer)
				if err := r.Update(ctx, &memberCluster); err != nil {
					clog.Error("add finalizers for cluster %v, failed: %v", memberCluster.Name, err)
					return ctrl.Result{}, err
				}
			}
		} else {
			if strslice.ContainsString(memberCluster.ObjectMeta.Finalizers, clusterFinalizer) {
				if err := r.deleteExternalResources(memberCluster, ctx); err != nil {
					// if fail to delete the external dependency here, return with error
					// so that it can be retried
					return ctrl.Result{}, err
				}
				// remove our finalizer from the list and update it.
				memberCluster.ObjectMeta.Finalizers = strslice.RemoveString(memberCluster.ObjectMeta.Finalizers, clusterFinalizer)
				if err := r.Update(ctx, &memberCluster); err != nil {
					clog.Error("remove finalizers for cluster %v, failed: %v", memberCluster.Name, err)
					return ctrl.Result{}, err
				}
			}
			// Stop reconciliation as the item is being deleted
			return ctrl.Result{}, nil
		}

		skip, err := addInternalCluster(memberCluster)
		if err != nil {
			clog.Error(err.Error())
		}
		if skip {
			return ctrl.Result{}, err
		}
		clog.Info("add cluster %v to internal clusters success", memberCluster)
	}

	// deploy resources to cluster
	err := deployResources(ctx, memberCluster, pivotCluster)
	if err != nil {
		log.Error("deploy resource failed: %v", err)
	}

	// start to scout loop for memberCluster warden, non-block
	err = multicluster.Interface().ScoutFor(context.Background(), memberCluster.Name)
	if err != nil {
		log.Error("start scout for memberCluster %v failed", memberCluster.Name)
	}

	// update cluster status to processing
	err = r.updateClusterStatus(ctx, memberCluster)
	if err != nil {
		log.Error("update cluster %v status failed", memberCluster.Name)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func SetupWithManager(mgr ctrl.Manager) error {
	r, err := newReconciler(mgr)
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1.Cluster{}).
		Complete(r)
}
