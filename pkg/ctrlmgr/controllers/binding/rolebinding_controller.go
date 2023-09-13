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

type RoleBindingReconciler struct {
	client.Client
}

func newRoleBindingReconciler(mgr manager.Manager) (*RoleBindingReconciler, error) {
	return &RoleBindingReconciler{
		Client: mgr.GetClient(),
	}, nil
}

func (r *RoleBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	roleBinding := &v1.RoleBinding{}
	if err := r.Get(ctx, req.NamespacedName, roleBinding); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	return r.syncUserOnCreate(ctx, roleBinding)
}

func (r *RoleBindingReconciler) syncUserOnCreate(ctx context.Context, roleBinding *v1.RoleBinding) (ctrl.Result, error) {
	userName, tenant, project, err := parseUserInRoleBinding(roleBinding.Name, roleBinding.Namespace)
	if err != nil {
		clog.Warn("parse RoleBinding(%s/%s) failed: %v", roleBinding.Name, roleBinding.Namespace, err)
		return ctrl.Result{}, nil
	}

	if len(userName+tenant+project) == 0 {
		// do nothing if name and namespace not matched
		return ctrl.Result{}, nil
	}

	user := &userv1.User{}
	err = r.Get(ctx, types.NamespacedName{Name: userName}, user)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		clog.Error("get user %v failed: %v", userName, err)
		return ctrl.Result{}, err
	}

	// add user relationship label to RoleBinding
	roleBinding.Labels = setBindingUserLabel(roleBinding.Labels, user.Name)
	err = updateRoleBinding(ctx, r.Client, roleBinding)
	if err != nil {
		return ctrl.Result{}, err
	}

	// add user scope bindings
	if len(tenant) > 0 {
		transition.AddUserScopeBindings(user, string(userv1.TenantScope), tenant, roleBinding.RoleRef.Name)
	} else if len(project) > 0 {
		transition.AddUserScopeBindings(user, string(userv1.ProjectScope), project, roleBinding.RoleRef.Name)
	}

	return ctrl.Result{}, transition.UpdateUserSpec(ctx, r.Client, user)
}

func SetupRoleBindingReconcilerWithManager(mgr ctrl.Manager, _ *options.Options) error {
	r, err := newRoleBindingReconciler(mgr)
	if err != nil {
		return err
	}

	predicateFunc := predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			_, ok := event.Object.GetLabels()[constants.LabelRelationship]
			if ok {
				// do not handle bindings that has processed
				return false
			}
			v, ok := event.Object.GetLabels()[constants.RbacLabel]
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
		For(&v1.RoleBinding{}).
		WithEventFilter(predicateFunc).
		Complete(r)
}
