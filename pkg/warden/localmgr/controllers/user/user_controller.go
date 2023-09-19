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
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	userv1 "github.com/kubecube-io/kubecube/pkg/apis/user/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/ctrlmgr/options"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/hash"
	"github.com/kubecube-io/kubecube/pkg/utils/transition"
)

const (
	// finalizerUser is used to clean up RoleBindings or ClusterRoleBindings which are under scope bindings
	finalizerUser = "user.finalizers.kubecube.io"
)

var _ reconcile.Reconciler = &UserReconciler{}

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func newReconciler(mgr manager.Manager) (*UserReconciler, error) {
	r := &UserReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	return r, nil
}

//+kubebuilder:rbac:groups=user.kubecube.io,resources=users,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=user.kubecube.io,resources=users/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=user.kubecube.io,resources=users/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the User object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.0/pkg/reconcile
func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	user := &userv1.User{}

	err := r.Get(ctx, req.NamespacedName, user)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		clog.Error("get user %v failed: %v", req.Name, err)
		return ctrl.Result{}, err
	}

	// user cr lifecycle is under warden control
	if user.DeletionTimestamp == nil {
		if err := r.ensureFinalizer(ctx, user); err != nil {
			clog.Error(err.Error())
			return ctrl.Result{}, err
		}
	} else {
		if err := r.removeFinalizer(ctx, user); err != nil {
			clog.Error(err.Error())
			return ctrl.Result{}, err
		}
		// stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	return r.syncUser(ctx, user)
}

func (r *UserReconciler) syncUser(ctx context.Context, user *userv1.User) (ctrl.Result, error) {
	err := r.refreshStatus(ctx, user)
	if err != nil {
		clog.Error(updateUserStatusErrStr(user.Name, err))
		return ctrl.Result{}, err
	}

	err = r.refreshBindings(ctx, user)
	if err != nil {
		clog.Error("refresh bindings failed: %v", err)
		return ctrl.Result{}, err
	}

	err = r.cleanOrphanBindings(ctx, user)
	if err != nil {
		clog.Error("clean up orphan bindings failed: %v", err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// refreshStatus refresh status according to scope bindings.
func (r *UserReconciler) refreshStatus(ctx context.Context, user *userv1.User) error {
	transition.RefreshUserStatus(user)
	return updateUserStatus(ctx, r.Client, user)
}

func (r *UserReconciler) cleanOrphanBindings(ctx context.Context, user *userv1.User) error {
	ls, err := labels.Parse(fmt.Sprintf("%v=%v", constants.LabelRelationship, user.Name))
	if err != nil {
		return err
	}

	bindingUnique := []string{}
	for _, binding := range user.Spec.ScopeBindings {
		bindingUnique = append(bindingUnique, transition.ScopeBindingUnique(binding))
	}

	bindingUniqueSet := sets.New[string](bindingUnique...)

	isTenantMember := len(user.Status.BelongTenants) > 0
	isProjectMember := len(user.Status.BelongProjects) > 0

	crbs := &v1.ClusterRoleBindingList{}
	err = r.List(ctx, crbs, &client.ListOptions{LabelSelector: ls})
	if err != nil {
		return err
	}
	for _, crb := range crbs.Items {
		if isGenBinding(crb.Name) && (isTenantMember || isProjectMember) {
			continue
		}
		scopeType, scopeName, role, _, err := transition.TransBinding(crb.Labels, crb.Subjects[0], crb.RoleRef)
		if err != nil {
			clog.Warn(err.Error())
			continue
		}
		if !bindingUniqueSet.Has(scopeName + scopeType + role) {
			clog.Info("clean up orphan ClusterRoleBinding (%v)", crb.Name)
			err = r.Delete(ctx, &crb)
			if err != nil && errors.IsNotFound(err) {
				return err
			}
		}
	}

	rbs := &v1.RoleBindingList{}
	err = r.List(ctx, rbs, &client.ListOptions{LabelSelector: ls})
	if err != nil {
		return err
	}
	for _, rb := range rbs.Items {
		scopeType, scopeName, role, _, err := transition.TransBinding(rb.Labels, rb.Subjects[0], rb.RoleRef)
		if err != nil {
			clog.Warn(err.Error())
			continue
		}
		if !bindingUniqueSet.Has(scopeName + scopeType + role) {
			clog.Info("clean up orphan RoleBinding (%v/%v)", rb.Name, rb.Namespace)
			err = r.Delete(ctx, &rb)
			if err != nil && errors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

// refreshBindings refresh related RoleBindings under binding scope.
func (r *UserReconciler) refreshBindings(ctx context.Context, user *userv1.User) error {
	var (
		errs                  []error
		needGenTenantBinding  bool
		needGenProjectBinding bool
	)

	// ignore any errors happen in refreshing, return all errors if had.
	for _, binding := range user.Spec.ScopeBindings {
		if binding.ScopeType == userv1.PlatformScope {
			errs = append(errs, r.refreshPlatformBinding(ctx, user.Name, binding))
		}
		if binding.ScopeType == userv1.TenantScope {
			needGenTenantBinding = true
			errs = append(errs, r.refreshNsBinding(ctx, user.Name, binding))
		}
		if binding.ScopeType == userv1.ProjectScope {
			needGenProjectBinding = true
			errs = append(errs, r.refreshNsBinding(ctx, user.Name, binding))
		}
	}

	if needGenTenantBinding {
		errs = append(errs, r.generateClusterRoleBinding(ctx, user.Name, userv1.TenantScope))
	}

	if needGenProjectBinding {
		errs = append(errs, r.generateClusterRoleBinding(ctx, user.Name, userv1.ProjectScope))
	}

	if len(errs) > 0 {
		// any error occurs when refreshing bindings will do retry
		return utilerrors.NewAggregate(errs)
	}

	return nil
}

// generateClusterRoleBinding will generate default build-in ClusterRoleBinding for user who belongs to tenant or project.
func (r *UserReconciler) generateClusterRoleBinding(ctx context.Context, user string, scopeType userv1.BindingScopeType) error {
	clusterRoleBinding := &v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				constants.RbacLabel:         constants.TrueStr,
				constants.LabelRelationship: user,
				constants.PlatformLabel:     constants.ClusterRolePlatform,
			},
		},
		Subjects: []v1.Subject{{
			APIGroup: constants.K8sGroupRBAC,
			Kind:     "User",
			Name:     user,
		}},
		RoleRef: v1.RoleRef{
			APIGroup: constants.K8sGroupRBAC,
			Kind:     constants.K8sKindClusterRole,
		},
	}

	if scopeType == userv1.TenantScope {
		clusterRoleBinding.RoleRef.Name = constants.TenantAdminCluster
	}
	if scopeType == userv1.ProjectScope {
		clusterRoleBinding.RoleRef.Name = constants.ProjectAdminCluster
	}

	clusterRoleBinding.Name = "gen-" + hash.GenerateBindingName(user, clusterRoleBinding.RoleRef.Name, "")

	return ignoreAlreadyExistErr(r.Create(ctx, clusterRoleBinding))
}

// refreshNsBinding refresh the RoleBinding of tenant or project under current cluster.
func (r *UserReconciler) refreshNsBinding(ctx context.Context, user string, binding userv1.ScopeBinding) error {
	namespaces, err := r.toFindNamespacesByScopeBinding(ctx, binding)
	if err != nil {
		return err
	}

	lb := map[string]string{
		constants.RbacLabel:         constants.TrueStr,
		constants.LabelRelationship: user,
	}

	if binding.ScopeType == userv1.TenantScope {
		lb[constants.TenantLabel] = binding.ScopeName
	}
	if binding.ScopeType == userv1.ProjectScope {
		lb[constants.ProjectLabel] = binding.ScopeName
	}

	var errs []error

	for _, ns := range namespaces {
		b := &v1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      hash.GenerateBindingName(user, binding.Role, ns.Name),
				Namespace: ns.Name,
				Labels:    lb,
				// we do not need warden sync here, every warden should process user event in self cluster
			},
			RoleRef: v1.RoleRef{
				APIGroup: constants.K8sGroupRBAC,
				Kind:     constants.KindClusterRole,
				Name:     binding.Role,
			},
			Subjects: []v1.Subject{
				{
					APIGroup: constants.K8sGroupRBAC,
					Kind:     "User",
					Name:     user,
				},
			},
		}
		errs = append(errs, ignoreAlreadyExistErr(r.Create(ctx, b)))
	}
	if len(errs) > 0 {
		// any error occurs when refreshing bindings will do retry
		return utilerrors.NewAggregate(errs)
	}

	return nil
}

// refreshPlatformBinding refresh the ClusterRoleBinding under current cluster.
func (r *UserReconciler) refreshPlatformBinding(ctx context.Context, user string, binding userv1.ScopeBinding) error {
	b := &v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: hash.GenerateBindingName(user, binding.Role, ""),
			Labels: map[string]string{
				constants.RbacLabel:         constants.TrueStr,
				constants.LabelRelationship: user,
				constants.PlatformLabel:     constants.ClusterRolePlatform,
			},
			// we do not need warden sync here, every warden should process user event in self cluster
		},
		RoleRef: v1.RoleRef{
			APIGroup: constants.K8sGroupRBAC,
			Kind:     constants.KindClusterRole,
			Name:     binding.Role,
		},
		Subjects: []v1.Subject{
			{
				APIGroup: constants.K8sGroupRBAC,
				Kind:     "User",
				Name:     user,
			},
		},
	}

	return ignoreAlreadyExistErr(r.Create(ctx, b))
}

// bindingsGc clean up RoleBindings or ClusterRoleBindings which are under scope bindings.
func (r *UserReconciler) bindingsGc(ctx context.Context, user string) error {
	ls, err := labels.Parse(fmt.Sprintf("%v=%v", constants.LabelRelationship, user))
	if err != nil {
		return err
	}

	crbs := &v1.ClusterRoleBindingList{}
	err = r.List(ctx, crbs, &client.ListOptions{LabelSelector: ls})
	if err != nil {
		return err
	}
	for _, crb := range crbs.Items {
		err = r.Delete(ctx, &crb)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	rbs := &v1.RoleBindingList{}
	err = r.List(ctx, rbs, &client.ListOptions{LabelSelector: ls})
	if err != nil {
		return err
	}
	for _, rb := range rbs.Items {
		err = r.Delete(ctx, &rb)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

// toFindNamespacesByScopeBinding will find namespaces under tenant or project
func (r *UserReconciler) toFindNamespacesByScopeBinding(ctx context.Context, binding userv1.ScopeBinding) ([]corev1.Namespace, error) {
	var labelSelectorStr string

	if binding.ScopeType == userv1.TenantScope {
		labelSelectorStr = fmt.Sprintf("%v=%v", constants.HncTenantLabel, binding.ScopeName)
	}

	if binding.ScopeType == userv1.ProjectScope {
		labelSelectorStr = fmt.Sprintf("%v=%v", constants.HncProjectLabel, binding.ScopeName)
	}

	ls, err := labels.Parse(labelSelectorStr)
	if err != nil {
		return nil, err
	}

	nsList := &corev1.NamespaceList{}
	err = r.List(ctx, nsList, &client.ListOptions{LabelSelector: ls})
	if err != nil {
		return nil, err
	}

	return nsList.Items, nil
}

func (r *UserReconciler) ensureFinalizer(ctx context.Context, user *userv1.User) error {
	if !controllerutil.ContainsFinalizer(user, finalizerUser) {
		controllerutil.AddFinalizer(user, finalizerUser)
		if err := r.Update(ctx, user); err != nil {
			// return err user reconcile retry
			clog.Error("add finalizers for cluster %v, failed: %v", user.Name, err)
			return err
		}
	}

	return nil
}

func (r *UserReconciler) removeFinalizer(ctx context.Context, user *userv1.User) error {
	if controllerutil.ContainsFinalizer(user, finalizerUser) {
		clog.Info("delete user %v and clean up for it", user.Name)

		err := r.bindingsGc(ctx, user.Name)
		if err != nil {
			// if fail to clean up bindings here, return with error
			// so that it can be retried
			return err
		}

		controllerutil.RemoveFinalizer(user, finalizerUser)

		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			newUser := &userv1.User{}
			err := r.Get(ctx, types.NamespacedName{Name: user.Name}, newUser)
			if err != nil {
				return err
			}

			newUser.Finalizers = user.Finalizers

			err = r.Update(ctx, newUser)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			clog.Warn("remove finalizers for cluster %v, failed: %v", user.Name, err)
			return err
		}
	}

	return nil
}

func ignoreAlreadyExistErr(err error) error {
	if errors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

// SetupWithManager sets up the controller with the Manager.
func SetupWithManager(mgr ctrl.Manager, _ *options.Options) error {
	r, err := newReconciler(mgr)
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&userv1.User{}).
		Watches(&corev1.Namespace{}, handler.EnqueueRequestsFromMapFunc(r.namespaceHandleFunc()), namespacePredicateFn).
		Complete(r)
}
