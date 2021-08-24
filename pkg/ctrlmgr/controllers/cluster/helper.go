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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	clusterv1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func createResource(ctx context.Context, obj client.Object, c client.Client, cluster string, objKind string) error {
	err := c.Create(ctx, obj, &client.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			log.Info("resource %v %v in cluster %v already exist: %v", objKind, obj.GetName(), cluster, err)
			return nil
		}
		return fmt.Errorf("deploy resource %v %v to cluster %v failed: %v", objKind, obj.GetName(), cluster, err)
	}

	log.Info("create %v %v to cluster %v success", objKind, obj.GetName(), cluster)

	return nil
}

// deleteExternalResources delete external dependency of cluster
func (r *ClusterReconciler) deleteExternalResources(cluster clusterv1.Cluster, ctx context.Context) error {
	//
	// delete any external resources associated with the cluster
	//
	// Ensure that delete implementation is idempotent and safe to invoke
	// multiple types for same object.

	// get target memberCluster client
	mClient := clients.Interface().Kubernetes(cluster.Name)

	// delete internal cluster and release goroutine inside
	err := multicluster.Interface().Del(cluster.Name)
	if err != nil {
		clog.Error("delete internal cluster %v failed", err)
		return err
	}
	clog.Debug("delete internal cluster %v success", cluster.Name)

	// delete kubecube-system of cluster
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: constants.CubeNamespace}}
	err = mClient.Direct().Delete(ctx, &ns)
	if err != nil {
		if errors.IsNotFound(err) {
			clog.Warn("namespace %v of cluster %v not failed, delete skip", constants.CubeNamespace, cluster.Name)
		}
		clog.Error("delete namespace %v of cluster %v failed: %v", constants.CubeNamespace, cluster.Name, err)
		return err
	}
	clog.Debug("delete kubecube-system of cluster %v success", cluster.Name)

	return nil
}

func (r *ClusterReconciler) ensureFinalizer(ctx context.Context, cluster *clusterv1.Cluster) error {
	if !controllerutil.ContainsFinalizer(cluster, clusterFinalizer) {
		controllerutil.AddFinalizer(cluster, clusterFinalizer)
		if err := r.Update(ctx, cluster); err != nil {
			clog.Error("add finalizers for cluster %v, failed: %v", cluster.Name, err)
			return err
		}
	}

	return nil
}

func (r *ClusterReconciler) removeFinalizer(ctx context.Context, cluster *clusterv1.Cluster) error {
	if controllerutil.ContainsFinalizer(cluster, clusterFinalizer) {
		clog.Info("delete cluster %v", cluster.Name)
		if err := r.deleteExternalResources(*cluster, ctx); err != nil {
			// if fail to delete the external dependency here, return with error
			// so that it can be retried
			return err
		}
		// remove our finalizer from the list and update it.
		controllerutil.RemoveFinalizer(cluster, clusterFinalizer)
		if err := r.Update(ctx, cluster); err != nil {
			clog.Error("remove finalizers for cluster %v, failed: %v", cluster.Name, err)
			return err
		}
	}

	return nil
}

func int32Ptr(i int32) *int32 { return &i }

func boolPtr(b bool) *bool { return &b }

func int64Ptr(i int64) *int64 { return &i }
