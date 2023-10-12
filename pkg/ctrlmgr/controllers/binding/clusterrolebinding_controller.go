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

	userv1 "github.com/kubecube-io/kubecube/pkg/apis/user/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/ctrlmgr/options"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/transition"
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
	clog.Debug("Reconcile ClusterRoleBinding(%v)", req.Name)
	clusterRoleBinding := &v1.ClusterRoleBinding{}
	if err := r.Get(ctx, req.NamespacedName, clusterRoleBinding); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	return r.syncUserOnCreate(ctx, clusterRoleBinding)
}

func (r *ClusterRoleBindingReconciler) syncUserOnCreate(ctx context.Context, clusterRoleBinding *v1.ClusterRoleBinding) (ctrl.Result, error) {
	userName, err := parseUserInClusterRoleBinding(clusterRoleBinding.Name)
	if err != nil {
		clog.Warn("parse ClusterRoleBinding %v failed: %v", clusterRoleBinding.Name, err)
		return ctrl.Result{}, nil
	}

	if userName == "" {
		return ctrl.Result{}, nil
	}

	foundUser := false

	user := &userv1.User{}
	err = r.Get(ctx, types.NamespacedName{Name: userName}, user)
	if err != nil {
		if errors.IsNotFound(err) {
			// give another chance to match username format {user}-{id}
			err = r.Get(ctx, types.NamespacedName{Name: parseUserNameWithID(userName)}, user)
			if err == nil {
				foundUser = true
			} else {
				foundUser = false
			}
		} else {
			clog.Error("get user %v failed: %v", userName, err)
			return ctrl.Result{}, err
		}
	} else {
		foundUser = true
	}

	if !foundUser {
		return ctrl.Result{}, nil
	}

	// update user scope binding
	transition.AddUserScopeBindings(user, string(userv1.PlatformScope), constants.ClusterRolePlatform, clusterRoleBinding.RoleRef.Name)
	err = transition.UpdateUserSpec(ctx, r.Client, user)
	if err != nil {
		return ctrl.Result{}, err
	}

	// add user relationship label
	clusterRoleBinding.Labels = setBindingUserLabel(clusterRoleBinding.Labels, user.Name)
	return ctrl.Result{}, updateClusterRoleBinding(ctx, r.Client, clusterRoleBinding)
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
			//_, ok = obj.GetLabels()[constants.LabelRelationship]
			//if ok {
			//	// do not handle bindings that has processed
			//	return false
			//}
			v, ok := obj.GetLabels()[constants.RbacLabel]
			if ok && v == "true" {
				return true
			}
			return false
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			// binding relationship can not be modified by update action, so ignore it.
			return false
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return false
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ClusterRoleBinding{}).
		WithEventFilter(predicateFunc).
		Complete(r)
}
