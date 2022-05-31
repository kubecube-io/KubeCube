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

package crds

import (
	"context"
	"reflect"

	"github.com/kubecube-io/kubecube/pkg/clog"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type CrdReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func newReconciler(mgr manager.Manager) (*CrdReconciler, error) {
	r := &CrdReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	return r, nil
}

func (r *CrdReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	clog.Info("Reconcile crd %v", req.Name)

	crd := v1.CustomResourceDefinition{}

	if err := r.Get(ctx, req.NamespacedName, &crd); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	return r.syncCrd(ctx, crd)
}

func (r *CrdReconciler) syncCrd(ctx context.Context, crd v1.CustomResourceDefinition) (ctrl.Result, error) {
	roles, err := r.mutateClusterRole(ctx, crd)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err = r.updateClusterRoles(ctx, roles); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// mutateClusterRole aggregate new crd auth fot build-in ClusterRole
func (r *CrdReconciler) mutateClusterRole(ctx context.Context, crd v1.CustomResourceDefinition) ([]*rbacv1.ClusterRole, error) {
	// ClusterRoles need to update
	var names []string
	var verbs []string
	resources := []string{crd.Spec.Names.Plural}

	if crd.Spec.Scope == "Cluster" {
		names = []string{"aggregate-to-platform-admin", "aggregate-to-reviewer", "aggregate-to-project-admin-cluster", "aggregate-to-tenant-admin-cluster"}
	} else {
		names = []string{"aggregate-to-platform-admin", "aggregate-to-reviewer", "aggregate-to-project-admin", "aggregate-to-tenant-admin"}
	}

	roles := make([]*rbacv1.ClusterRole, 0, len(names))

	for _, name := range names {
		role := &rbacv1.ClusterRole{}
		if err := r.Get(ctx, types.NamespacedName{Name: name}, role); err != nil {
			return nil, err
		}
		if name == "aggregate-to-platform-admin" || name == "aggregate-to-project-admin" || name == "aggregate-to-tenant-admin" {
			verbs = []string{"get", "list", "watch", "create", "delete", "deletecollection", "update", "patch"}
		}
		if name == "aggregate-to-reviewer" || name == "aggregate-to-project-admin-cluster" || name == "aggregate-to-tenant-admin-cluster" {
			verbs = []string{"get", "list", "watch"}
		}
		newPolicyRule := makePolicyRule(resources, nil, verbs)
		role.Rules = insertRules(role.Rules, newPolicyRule)
		roles = append(roles, role)
	}

	return roles, nil
}

func (r *CrdReconciler) updateClusterRoles(ctx context.Context, clusterRoles []*rbacv1.ClusterRole) error {
	for _, clusterRole := range clusterRoles {
		if err := r.Update(ctx, clusterRole); err != nil {
			return err
		}
	}
	return nil
}

func makePolicyRule(resources, resourceNames, verbs []string) rbacv1.PolicyRule {
	return rbacv1.PolicyRule{
		APIGroups:     []string{"*"},
		Resources:     resources,
		ResourceNames: resourceNames,
		Verbs:         verbs,
	}
}

func insertRules(rules []rbacv1.PolicyRule, newRule rbacv1.PolicyRule) []rbacv1.PolicyRule {
	for _, rule := range rules {
		if reflect.DeepEqual(rule, newRule) {
			return rules
		}
	}

	rules = append(rules, newRule)
	return rules
}

// SetupWithManager sets up the controller with the Manager.
func SetupWithManager(mgr ctrl.Manager) error {
	r, err := newReconciler(mgr)
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.CustomResourceDefinition{}).
		Complete(r)
}
