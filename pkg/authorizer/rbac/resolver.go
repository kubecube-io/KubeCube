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

package rbac

import (
	"fmt"

	userv1 "github.com/kubecube-io/kubecube/pkg/apis/user/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/authentication/user"
)

type RoleResolver interface {
	// RolesFor get all of roles and cluster roles bind to user, with non empty
	// namespace will match both Role and ClusterRole, otherwise only clusterRole
	// will be matched.
	RolesFor(user user.Info, namespace string) ([]*rbacv1.Role, []*rbacv1.ClusterRole, error)

	// UsersFor get all of users bind to role reference, if Role with namespace,
	// will match RoleBindings and ClusterRoleBindings, otherwise only Cluster
	// will be matched.
	UsersFor(role rbacv1.RoleRef, namespace string) ([]*userv1.User, error)

	// VisitRulesFor invokes visitor() with each rule that applies to a given user in a given namespace,
	// and each error encountered resolving those rules. Rule may be nil if err is non-nil.
	// If visitor() returns false, visiting is short-circuited.
	VisitRulesFor(user user.Info, namespace string, visitor func(source fmt.Stringer, rule *rbacv1.PolicyRule, err error) bool)
}

// resultAccumulator accumulate match result and error in processing
type resultAccumulator struct {
	roles        []*rbacv1.Role
	clusterRoles []*rbacv1.ClusterRole
	users        []*userv1.User
	errors       []error
}

type visitor func(role *rbacv1.Role, clusterRole *rbacv1.ClusterRole, user *userv1.User, err error)

func (r *resultAccumulator) visit(role *rbacv1.Role, clusterRole *rbacv1.ClusterRole, user *userv1.User, err error) {
	if role != nil {
		for _, v := range r.roles {
			if v.Name == role.Name {
				return
			}
		}
		r.roles = append(r.roles, role)
	}
	if clusterRole != nil {
		for _, v := range r.clusterRoles {
			if v.Name == clusterRole.Name {
				return
			}
		}
		r.clusterRoles = append(r.clusterRoles, clusterRole)
	}
	if user != nil {
		for _, v := range r.users {
			if v.Name == user.Name {
				return
			}
		}
		r.users = append(r.users, user)
	}
	if err != nil {
		r.errors = append(r.errors, err)
	}
	return
}

func (r *DefaultResolver) RolesFor(user user.Info, namespace string) ([]*rbacv1.Role, []*rbacv1.ClusterRole, error) {
	visitor := &resultAccumulator{}
	r.VisitRolesFor(user, namespace, visitor.visit)
	return visitor.roles, visitor.clusterRoles, utilerrors.NewAggregate(visitor.errors)
}

func (r *DefaultResolver) UsersFor(role rbacv1.RoleRef, namespace string) ([]*userv1.User, error) {
	visitor := &resultAccumulator{}
	r.VisitUsersFor(role, namespace, visitor.visit)
	return visitor.users, utilerrors.NewAggregate(visitor.errors)
}

func (r *DefaultResolver) matchClusterRoleBindingsForUser(role rbacv1.RoleRef, visitor visitor) {
	if clusterRoleBindings, err := r.ListClusterRoleBindings(); err != nil {
		visitor(nil, nil, nil, err)
	} else {
		for _, clusterRoleBinding := range clusterRoleBindings {
			if clusterRoleBinding.RoleRef != role {
				continue
			}
			for _, subject := range clusterRoleBinding.Subjects {
				u, err := r.GetUser(subject.Name)
				if err != nil {
					visitor(nil, nil, nil, err)
					continue
				}
				visitor(nil, nil, &u, nil)
			}
		}
	}
}

func (r *DefaultResolver) matchRoleBindingsForUser(role rbacv1.RoleRef, namespace string, visitor visitor) {
	if len(namespace) > 0 {
		if roleBindings, err := r.ListRoleBindings(namespace); err != nil {
			visitor(nil, nil, nil, err)
		} else {
			for _, roleBinding := range roleBindings {
				if roleBinding.RoleRef != role {
					continue
				}
				for _, subject := range roleBinding.Subjects {
					u, err := r.GetUser(subject.Name)
					if err != nil {
						visitor(nil, nil, nil, err)
						continue
					}
					visitor(nil, nil, &u, nil)
				}
			}
		}
	}
}

func (r *DefaultResolver) VisitUsersFor(role rbacv1.RoleRef, namespace string, visitor visitor) {
	switch role.Kind {
	case "ClusterRole":
		// search clusterRoleBindings and RoleBindings
		r.matchClusterRoleBindingsForUser(role, visitor)
		r.matchRoleBindingsForUser(role, namespace, visitor)
	case "Role":
		// only search RoleBindings
		r.matchRoleBindingsForUser(role, namespace, visitor)
	default:
		return
	}
}

func (r *DefaultResolver) VisitRolesFor(user user.Info, namespace string, visitor visitor) {
	if clusterRoleBindings, err := r.ListClusterRoleBindings(); err != nil {
		visitor(nil, nil, nil, err)
	} else {
		for _, clusterRoleBinding := range clusterRoleBindings {
			_, applies := appliesTo(user, clusterRoleBinding.Subjects, "")
			if !applies {
				continue
			}
			clusterRole, err := r.GetClusterRole(clusterRoleBinding.RoleRef.Name)
			if err != nil {
				visitor(nil, nil, nil, err)
				continue
			}
			visitor(nil, &clusterRole, nil, nil)
		}
	}

	if len(namespace) > 0 {
		if roleBindings, err := r.ListRoleBindings(namespace); err != nil {
			visitor(nil, nil, nil, err)
		} else {
			for _, roleBinding := range roleBindings {
				_, applies := appliesTo(user, roleBinding.Subjects, namespace)
				if !applies {
					continue
				}
				switch roleBinding.RoleRef.Kind {
				case "Role":
					role, err := r.GetRole(namespace, roleBinding.RoleRef.Name)
					if err != nil {
						visitor(nil, nil, nil, err)
						continue
					}
					visitor(&role, nil, nil, nil)
				case "ClusterRole":
					clusterRole, err := r.GetClusterRole(roleBinding.RoleRef.Name)
					if err != nil {
						visitor(nil, nil, nil, err)
						continue
					}
					visitor(nil, &clusterRole, nil, nil)
				default:
					continue
				}
			}

		}
	}
}

// appliesTo returns whether any of the bindingSubjects applies to the specified subject,
// and if true, the index of the first subject that applies
func appliesTo(user user.Info, bindingSubjects []rbacv1.Subject, namespace string) (int, bool) {
	for i, bindingSubject := range bindingSubjects {
		if appliesToUser(user, bindingSubject, namespace) {
			return i, true
		}
	}
	return 0, false
}

// appliesToUser only support user kind now
func appliesToUser(user user.Info, subject rbacv1.Subject, namespace string) bool {
	switch subject.Kind {
	case rbacv1.UserKind:
		return user.GetName() == subject.Name

	case rbacv1.GroupKind:
		// group not used here
		return false

	case rbacv1.ServiceAccountKind:
		// service account not used here
		return false
	default:
		return false
	}
}
