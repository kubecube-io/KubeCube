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

	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
)

type CrdReconciler struct {
	client.Client
	pivotClient client.Client
	Scheme      *runtime.Scheme
}

func newReconciler(mgr manager.Manager, pivotClient client.Client) (*CrdReconciler, error) {
	r := &CrdReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		pivotClient: pivotClient,
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
		names = []string{constants.AggPlatformAdmin, constants.AggReviewer, constants.AggProjectAdminCluster, constants.AggTenantAdminCluster}
	} else {
		names = []string{constants.AggPlatformAdmin, constants.AggReviewer, constants.AggProjectAdmin, constants.AggTenantAdmin}
	}

	roles := make([]*rbacv1.ClusterRole, 0, len(names))

	for _, name := range names {
		role := &rbacv1.ClusterRole{}
		if err := r.pivotClient.Get(ctx, types.NamespacedName{Name: name}, role); err != nil {
			return nil, err
		}
		if name == constants.AggPlatformAdmin || name == constants.AggProjectAdmin || name == constants.AggTenantAdmin {
			verbs = []string{constants.GetVerb, constants.ListVerb, constants.WatchVerb, constants.CreateVerb, constants.DeleteVerb, constants.DeleteCollectionVerb, constants.UpdateVerb, constants.PatchVerb}
		}
		if name == constants.AggReviewer || name == constants.AggProjectAdminCluster || name == constants.AggTenantAdminCluster {
			verbs = []string{constants.GetVerb, constants.ListVerb, constants.WatchVerb}
		}
		newPolicyRule := makePolicyRule(resources, nil, verbs)
		role.Rules = insertRules(role.Rules, newPolicyRule)
		roles = append(roles, role)
	}

	return roles, nil
}

func (r *CrdReconciler) updateClusterRoles(ctx context.Context, clusterRoles []*rbacv1.ClusterRole) error {
	for _, clusterRole := range clusterRoles {
		if err := r.pivotClient.Update(ctx, clusterRole); err != nil {
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
		for _, resource := range rule.Resources {
			if len(newRule.Resources) == 1 && newRule.Resources[0] == resource {
				return rules
			}
		}
	}

	rules = append(rules, newRule)
	return rules
}

// SetupWithManager sets up the controller with the Manager.
func SetupWithManager(mgr ctrl.Manager, pivotClient client.Client) error {
	r, err := newReconciler(mgr, pivotClient)
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.CustomResourceDefinition{}).
		Complete(r)
}
