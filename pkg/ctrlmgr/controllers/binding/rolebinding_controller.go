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
			return r.syncUserOnDelete(ctx, req.Name, req.Namespace)
		}
		return ctrl.Result{}, err
	}

	return r.syncUserOnCreate(ctx, roleBinding)
}

func (r *RoleBindingReconciler) syncUserOnCreate(ctx context.Context, roleBinding *v1.RoleBinding) (ctrl.Result, error) {
	userName, tenant, project, err := parseUserInRoleBinding(roleBinding.Name, roleBinding.Namespace)
	if err != nil {
		clog.Error("parse RoleBinding(%s/%s) failed: %v", roleBinding.Name, roleBinding.Namespace, err)
		return ctrl.Result{}, err
	}

	if len(userName+tenant+project) == 0 {
		// do nothing if name and namespace not matched
		return ctrl.Result{}, nil
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

	if len(tenant) > 0 {
		err = updateUserStatus(ctx, r.Client, user, constants.ClusterRoleTenant)
		if err != nil {
			clog.Error("update user %v status failed: %v", user, err)
			return ctrl.Result{}, err
		}
	} else if len(project) > 0 {
		addUserToProject(user, project)
		err = updateUserStatus(ctx, r.Client, user, constants.ClusterRoleProject)
		if err != nil {
			clog.Error("update user %v status failed: %v", user, err)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *RoleBindingReconciler) syncUserOnDelete(ctx context.Context, name, namespace string) (ctrl.Result, error) {
	userName, tenant, project, err := parseUserInRoleBinding(name, namespace)
	if err != nil {
		clog.Error("parse RoleBinding(%s/%s) failed: %v", name, namespace, err)
		return ctrl.Result{}, err
	}

	if len(userName+tenant+project) == 0 {
		// do nothing if name and namespace not matched
		return ctrl.Result{}, nil
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

	if len(tenant) > 0 {
		err = updateUserStatus(ctx, r.Client, user, constants.ClusterRoleTenant)
		if err != nil {
			clog.Error("update user %v status failed: %v", user, err)
			return ctrl.Result{}, err
		}
	} else if len(project) > 0 {
		moveUserFromProject(user, project)
		err = updateUserStatus(ctx, r.Client, user, constants.ClusterRoleProject)
		if err != nil {
			clog.Error("update user %v status failed: %v", user, err)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func SetupRoleBindingReconcilerWithManager(mgr ctrl.Manager, _ *options.Options) error {
	r, err := newRoleBindingReconciler(mgr)
	if err != nil {
		return err
	}

	predicateFunc := predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
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
			v, ok := deleteEvent.Object.GetLabels()[constants.RbacLabel]
			if ok && v == "true" {
				return true
			}
			return false
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.RoleBinding{}).
		WithEventFilter(predicateFunc).
		Complete(r)
}
