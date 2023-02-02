/*
Copyright 2022 KubeCube Authors

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

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/quota"
	"github.com/kubecube-io/kubecube/pkg/quota/k8s"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
)

type QuotaReconciler struct {
	client.Client
	pivotClient client.Client
	Scheme      *runtime.Scheme
}

func newReconciler(mgr manager.Manager, pivotClient client.Client) (*QuotaReconciler, error) {
	r := &QuotaReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		pivotClient: pivotClient,
	}
	return r, nil
}

func (r *QuotaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	clog.Info("Reconcile ResourceQuota (%v/%v)", req.Name, req.Namespace)

	currentQuota := &v1.ResourceQuota{}
	err := r.Get(ctx, req.NamespacedName, currentQuota)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		clog.Error("get ResourceQuota (%v/%v) failed: %v", req.Name, req.Namespace, err)
		return ctrl.Result{}, err
	}

	quotaOperator := k8s.NewQuotaOperator(r.pivotClient, r.Client, currentQuota, nil, ctx)

	if currentQuota.DeletionTimestamp == nil {
		if err := r.ensureFinalizer(ctx, currentQuota); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		if err := r.removeFinalizer(ctx, currentQuota, quotaOperator); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, quotaOperator.UpdateParentStatus(false)
}

func (r *QuotaReconciler) ensureFinalizer(ctx context.Context, currentQuota *v1.ResourceQuota) error {
	if !controllerutil.ContainsFinalizer(currentQuota, quota.Finalizer) {
		controllerutil.AddFinalizer(currentQuota, quota.Finalizer)
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			newQuota := &v1.ResourceQuota{}
			err := r.Get(ctx, types.NamespacedName{Name: currentQuota.Name, Namespace: currentQuota.Namespace}, newQuota)
			if err != nil {
				return err
			}
			newQuota.Finalizers = currentQuota.Finalizers
			err = r.Update(ctx, newQuota)
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			clog.Warn("add finalizer to ResourceQuota (%v/%v) failed: %v", currentQuota.Name, currentQuota.Namespace, err)
			return err
		}
	}

	return nil
}

func (r *QuotaReconciler) removeFinalizer(ctx context.Context, currentQuota *v1.ResourceQuota, quotaOperator quota.Interface) error {
	if controllerutil.ContainsFinalizer(currentQuota, quota.Finalizer) {
		clog.Info("delete ResourceQuota (%v/%v)", currentQuota.Name, currentQuota.Namespace)
		err := quotaOperator.UpdateParentStatus(true)
		if err != nil {
			clog.Error("update parent status of ResourceQuota (%v/%v) failed: %v", currentQuota.Name, currentQuota.Namespace, err)
			return err
		}
		controllerutil.RemoveFinalizer(currentQuota, quota.Finalizer)
		err = r.Update(ctx, currentQuota)
		if err != nil {
			clog.Warn("delete finalizer to ResourceQuota (%v/%v) failed: %v", currentQuota.Name, currentQuota.Namespace, err)
			return err
		}
	}

	return nil
}

func isManagedResourceQuota(obj client.Object) bool {
	labels := obj.GetLabels()
	if labels == nil {
		return false
	}
	if _, ok := labels[constants.CubeQuotaLabel]; ok {
		return true
	}
	return false
}

// SetupWithManager sets up the controller with the Manager.
func SetupWithManager(mgr ctrl.Manager, pivotClient client.Client) error {
	r, err := newReconciler(mgr, pivotClient)
	if err != nil {
		return err
	}

	// filter update event
	predicateFunc := predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			return isManagedResourceQuota(event.Object)
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			oldObj, ok := updateEvent.ObjectOld.(*v1.ResourceQuota)
			if !ok {
				return false
			}
			newObj, ok := updateEvent.ObjectNew.(*v1.ResourceQuota)
			if !ok {
				return false
			}
			if oldObj.DeletionTimestamp != nil || newObj.DeletionTimestamp != nil {
				return isManagedResourceQuota(updateEvent.ObjectNew)
			}
			if reflect.DeepEqual(oldObj.Spec, newObj.Spec) {
				return false
			}
			return isManagedResourceQuota(updateEvent.ObjectNew)
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return isManagedResourceQuota(deleteEvent.Object)
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ResourceQuota{}).
		WithEventFilter(predicateFunc).
		Complete(r)
}
