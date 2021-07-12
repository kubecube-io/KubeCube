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

package quota

import (
	"context"
	"reflect"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"

	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/quota/cube"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	quotav1 "github.com/kubecube-io/kubecube/pkg/apis/quota/v1"
)

var (
	log clog.CubeLogger

	_ reconcile.Reconciler = &CubeResourceQuotaReconciler{}
)

// CubeResourceQuotaReconciler reconciles a CubeResourceQuota object
type CubeResourceQuotaReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func newReconciler(mgr manager.Manager) (reconcile.Reconciler, error) {
	log = clog.WithName("cubeResourceQuota")

	r := &CubeResourceQuotaReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	return r, nil
}

//+kubebuilder:rbac:groups=quota.kubecube.io,resources=cuberesourcequota,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=quota.kubecube.io,resources=cuberesourcequota/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=quota.kubecube.io,resources=cuberesourcequota/finalizers,verbs=update

// Reconcile of cube resource quota only used for initializing status of cube resource quota
func (r *CubeResourceQuotaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	quota := quotav1.CubeResourceQuota{}
	err := r.Get(ctx, req.NamespacedName, &quota)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// init status of cube resource quota when create
	if quota.Status.Used == nil && quota.Status.Hard == nil {
		log.Info("initialize status of cube resource quota: %v, target: %+v", quota.Name, quota.Spec.Target)
		err = r.initQuotaStatus(ctx, &quota)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	if !reflect.DeepEqual(quota.Spec.Hard, quota.Status.Hard) {
		quotaCopy := quota.DeepCopy()

		quotaCopy.Status.Hard = quotaCopy.Spec.Hard

		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			err := r.Status().Update(ctx, quotaCopy, &client.UpdateOptions{})
			if !errors.IsConflict(err) {
				return err
			}
			return nil
		})
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *CubeResourceQuotaReconciler) initQuotaStatus(ctx context.Context, quota *quotav1.CubeResourceQuota) error {
	quotaCopy := quota.DeepCopy()

	cube.InitStatus(quotaCopy)

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := r.Status().Update(ctx, quotaCopy, &client.UpdateOptions{})
		if !errors.IsConflict(err) {
			return err
		}
		return nil
	})
}

// SetupWithManager sets up the controller with the Manager.
func SetupWithManager(mgr ctrl.Manager) error {
	r, err := newReconciler(mgr)
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&quotav1.CubeResourceQuota{}).
		Complete(r)
}
