package transition

import (
	"context"
	"fmt"

	userv1 "github.com/kubecube-io/kubecube/pkg/apis/user/v1"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/hierarchical-namespaces/api/v1alpha2"
)

func SubNs2Ns(subNs *v1alpha2.SubnamespaceAnchor) *v1.Namespace {
	if subNs.Labels == nil {
		return nil
	}

	tenant := subNs.Labels[constants.TenantLabel]
	project := subNs.Labels[constants.ProjectLabel]

	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: subNs.Name,
			Annotations: map[string]string{
				constants.HncAnnotation: subNs.Namespace,
			},
			Labels: map[string]string{
				constants.HncIncludedNsLabel: "true",
				fmt.Sprintf("%v%v.tree.hnc.x-k8s.io/depth", constants.ProjectNsPrefix, project): constants.HncProjectDepth,
				fmt.Sprintf("%v%v.tree.hnc.x-k8s.io/depth", constants.TenantNsPrefix, tenant):   constants.HncTenantDepth,
				fmt.Sprintf("%v.tree.hnc.x-k8s.io/depth", subNs.Name):                           constants.HncCurrentDepth,
				constants.HncProjectLabel: project,
				constants.HncTenantLabel:  tenant,
			},
		},
	}

	return ns
}

func TransBinding(labels map[string]string, sub rbacv1.Subject, ref rbacv1.RoleRef) (scopeType string, scopeName string, role string, user string, err error) {
	if labels == nil {
		err = fmt.Errorf("empty labels")
		return
	}

	tenant := labels[constants.TenantLabel]
	project := labels[constants.ProjectLabel]

	if len(tenant) > 0 {
		scopeType = constants.ClusterRoleTenant
		scopeName = tenant
		role = ref.Name
		user = sub.Name
		return
	}

	if len(project) > 0 {
		scopeType = constants.ClusterRoleProject
		scopeName = project
		role = ref.Name
		user = sub.Name
		return
	}

	err = fmt.Errorf("invalid labels")
	return
}

func AddUserScopeBindings(user *userv1.User, scopeType, scopeName, role string) {
	bindingUnique := []string{}
	for _, binding := range user.Spec.ScopeBindings {
		bindingUnique = append(bindingUnique, ScopeBindingUnique(binding))
	}

	bindingUniqueSet := sets.NewString(bindingUnique...)

	if !bindingUniqueSet.Has(scopeName + scopeType + role) {
		user.Spec.ScopeBindings = append(user.Spec.ScopeBindings, userv1.ScopeBinding{
			ScopeName: scopeName,
			ScopeType: userv1.BindingScopeType(scopeType),
			Role:      role,
		})
	}
}

func RemoveUserScopeBindings(user *userv1.User, scopeType, scopeName, role string) {
	newScopeBindings := []userv1.ScopeBinding{}
	for _, binding := range user.Spec.ScopeBindings {
		if binding.ScopeName == scopeName && string(binding.ScopeType) == scopeType && binding.Role == role {
			continue
		}
		newScopeBindings = append(newScopeBindings, binding)
	}
	user.Spec.ScopeBindings = newScopeBindings
}

func ScopeBindingUnique(b userv1.ScopeBinding) string {
	return b.ScopeName + string(b.ScopeType) + b.Role
}

func UpdateUserSpec(ctx context.Context, cli client.Client, user *userv1.User) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		newUser := &userv1.User{}
		err := cli.Get(ctx, types.NamespacedName{Name: user.Name}, newUser)
		if err != nil {
			return err
		}

		newUser.Spec = user.Spec

		err = cli.Status().Update(ctx, newUser)
		if err != nil {
			return err
		}
		return nil
	})
}
