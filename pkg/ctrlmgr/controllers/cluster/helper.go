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
	"reflect"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	clusterv1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	"github.com/kubecube-io/kubecube/pkg/utils"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/env"
	"github.com/kubecube-io/kubecube/pkg/utils/kubeconfig"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func createResource(ctx context.Context, obj client.Object, c client.Client, cluster string, objKind string) error {
	if reflect.ValueOf(obj).IsNil() {
		return fmt.Errorf("object can not be nil")
	}

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

// waitForJobComplete block and wait until job meets completed
func waitForJobComplete(cli client.Client, namespacedName types.NamespacedName) error {
	isJobCompleted := func(j v1.Job) bool {
		status := j.Status
		for _, c := range status.Conditions {
			if c.Status == corev1.ConditionTrue && c.Type == "Complete" {
				return true
			}
		}
		return false
	}

	return wait.Poll(3*time.Second, 5*time.Minute, func() (done bool, err error) {
		j := v1.Job{}
		err = cli.Get(context.Background(), namespacedName, &j)
		if err != nil {
			// fetch job failed, abort and return error directly
			return false, err
		}
		if isJobCompleted(j) {
			clog.Info("install dependence job(%v/%v) meet completed", namespacedName.Namespace, namespacedName.Name)
			return true, nil
		}
		return false, nil
	})
}

func tryConnectCluster(cluster clusterv1.Cluster) (client.Client, error) {
	config, err := kubeconfig.LoadKubeConfigFromBytes(cluster.Spec.KubeConfig)
	if err != nil {
		return nil, err
	}

	cli, err := client.New(config, client.Options{})
	if err != nil {
		return nil, err
	}

	return cli, nil
}

// isDatingCluster tells if the cluster is dating with KubeCube
func isDatingCluster(ctx context.Context, cli client.Client, cluster string) bool {
	warden := appsv1.Deployment{}
	err := cli.Get(ctx, client.ObjectKey{Name: constants.Warden, Namespace: env.CubeNamespace()}, &warden)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Debug("warden not found in cluster %v", cluster)
			return false
		}
		clog.Warn("confirm warden in cluster %v failed, try redeploy", cluster)
		return false
	}

	clog.Info("warden of cluster %v is dating with kubecube", cluster)
	return true
}

// deleteExternalResources delete external dependency of cluster
func (r *ClusterReconciler) deleteExternalResources(cluster clusterv1.Cluster, ctx context.Context) error {
	//
	// delete any external resources associated with the cluster
	//
	// Ensure that delete implementation is idempotent and safe to invoke
	// multiple types for same object.

	// reconnected failed cluster do not need gc
	reconnectedFailedState := clusterv1.ClusterReconnectedFailed
	if cluster.Status.State == &reconnectedFailedState {
		return nil
	}

	// stop retry task if cluster in retry queue
	cancel, ok := r.retryQueue.Load(cluster.Name)
	if ok {
		cancel.(context.CancelFunc)()
		clog.Debug("stop retry task of cluster %v success", cluster.Name)
		return nil
	}

	// get target memberCluster client
	internalCluster, err := multicluster.Interface().Get(cluster.Name)
	if internalCluster != nil && err != nil {
		// retry if member cluster is unhealthy
		clog.Warn(err.Error())
		return err
	}
	if internalCluster == nil {
		clog.Warn("cluster %v may be deleted, fallthrough", cluster.Name)
		return nil
	}

	if !env.RetainMemberClusterResource() {
		mClient := internalCluster.Client
		// delete kubecube-system of cluster
		ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: env.CubeNamespace()}}
		err := mClient.Direct().Delete(ctx, &ns)
		if err != nil {
			if errors.IsNotFound(err) {
				clog.Warn("namespace %v of cluster %v not found, delete skip", env.CubeNamespace(), cluster.Name)
			}
			// retry if delete resources in member cluster failed
			clog.Error("delete namespace %v of cluster %v failed: %v", env.CubeNamespace(), cluster.Name, err)
			return err
		}
	}

	// delete internal cluster and release goroutine inside
	err = multicluster.Interface().Del(cluster.Name)
	if err != nil {
		clog.Warn("cluster %v not found in internal clusters, skip", err)
	}

	clog.Debug("delete kubecube of cluster %v success", cluster.Name)

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

		err := utils.UpdateClusterStatusByState(ctx, r.Client, cluster, clusterv1.ClusterDeleting)
		if err != nil {
			return err
		}

		if err := r.deleteExternalResources(*cluster, ctx); err != nil {
			// if fail to delete the external dependency here, return with error
			// so that it can be retried
			return err
		}
		// remove our finalizer from the list and update it.
		controllerutil.RemoveFinalizer(cluster, clusterFinalizer)
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			newCluster := &clusterv1.Cluster{}
			err := r.Get(ctx, types.NamespacedName{Name: cluster.Name}, newCluster)
			if err != nil {
				return err
			}

			newCluster.Finalizers = cluster.Finalizers

			err = r.Update(ctx, newCluster)
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			clog.Warn("remove finalizers for cluster %v, failed: %v", cluster.Name, err)
			return err
		}
	}

	return nil
}

func int32Ptr(i int32) *int32 { return &i }

func boolPtr(b bool) *bool { return &b }

func int64Ptr(i int64) *int64 { return &i }
