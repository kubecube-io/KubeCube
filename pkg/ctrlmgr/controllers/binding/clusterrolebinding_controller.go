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

package binding

import (
	"context"

	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v12 "github.com/kubecube-io/kubecube/pkg/apis/user/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/ctrlmgr/options"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
)

type ClusterRoleBindingReconciler struct {
	client.Client
}

func newClusterRoleBindingReconciler(mgr manager.Manager) (*ClusterRoleBindingReconciler, error) {
	return &ClusterRoleBindingReconciler{
		Client: mgr.GetClient(),
	}, nil
}

func (r *ClusterRoleBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	clusterRoleBinding := &v1.ClusterRoleBinding{}
	if err := r.Get(ctx, req.NamespacedName, clusterRoleBinding); err != nil {
		if errors.IsNotFound(err) {
			return r.syncUserOnDelete(ctx, req.Name)
		}
		return ctrl.Result{}, err
	}

	return r.syncUserOnCreate(ctx, clusterRoleBinding)
}

func (r *ClusterRoleBindingReconciler) syncUserOnCreate(ctx context.Context, clusterRoleBinding *v1.ClusterRoleBinding) (ctrl.Result, error) {
	userName, err := parseUserInClusterRoleBinding(clusterRoleBinding.Name)
	if err != nil {
		clog.Error("parse ClusterRoleBinding %v failed: %v", clusterRoleBinding.Name, err)
		return ctrl.Result{}, err
	}

	user := &v12.User{}
	err = r.Get(ctx, types.NamespacedName{Name: userName}, user)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		clog.Error("get user %v failed: %v", userName, err)
		return ctrl.Result{}, err
	}

	appointUserAdmin(user)

	err = updateUserStatus(ctx, r.Client, user)
	if err != nil {
		clog.Error("update user %v status failed: %v", user, err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ClusterRoleBindingReconciler) syncUserOnDelete(ctx context.Context, name string) (ctrl.Result, error) {
	userName, err := parseUserInClusterRoleBinding(name)
	if err != nil {
		clog.Error("parse ClusterRoleBinding %v failed: %v", name, err)
		return ctrl.Result{}, err
	}

	user := &v12.User{}
	err = r.Get(ctx, types.NamespacedName{Name: userName}, user)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		clog.Error("get user %v failed: %v", userName, err)
		return ctrl.Result{}, err
	}

	impeachUserAdmin(user)

	err = updateUserStatus(ctx, r.Client, user)
	if err != nil {
		clog.Error("update user %v status failed: %v", user, err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func SetupClusterRoleBindingReconcilerWithManager(mgr ctrl.Manager, _ *options.Options) error {
	r, err := newClusterRoleBindingReconciler(mgr)
	if err != nil {
		return err
	}

	predicateFunc := predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			obj, ok := event.Object.(*v1.ClusterRoleBinding)
			if !ok {
				return false
			}
			if obj.RoleRef.Kind == constants.KindClusterRole && obj.RoleRef.Name == constants.PlatformAdmin {
				return true
			}
			return false
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			// binding relationship can not be modified by update action, so ignore it.
			return false
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			obj, ok := deleteEvent.Object.(*v1.ClusterRoleBinding)
			if !ok {
				return false
			}
			if obj.RoleRef.Kind == constants.KindClusterRole && obj.RoleRef.Name == constants.PlatformAdmin {
				return true
			}
			return false
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ClusterRoleBinding{}).
		WithEventFilter(predicateFunc).
		Complete(r)
}
