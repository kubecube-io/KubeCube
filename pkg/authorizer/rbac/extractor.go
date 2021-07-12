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
	"context"

	userv1 "github.com/kubecube-io/kubecube/pkg/apis/user/v1"

	"github.com/kubecube-io/kubecube/pkg/clients"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

type RoleExtractor interface {
	GetUser(name string) (userv1.User, error)
	ListUser() ([]userv1.User, error)

	GetRole(namespace, name string) (rbacv1.Role, error)
	ListRoleBindings(namespace string) ([]rbacv1.RoleBinding, error)

	GetClusterRole(name string) (rbacv1.ClusterRole, error)
	ListClusterRoleBindings() ([]rbacv1.ClusterRoleBinding, error)
}

// DefaultResolver resolve rules of rbac
type DefaultResolver struct {
	// resources get or list from cache
	// sync by informer
	cache.Cache
}

func NewDefaultResolver(cluster string) *DefaultResolver {
	c := clients.Interface().Kubernetes(cluster).Cache()
	return &DefaultResolver{Cache: c}
}

func (r *DefaultResolver) GetUser(name string) (userv1.User, error) {
	key := types.NamespacedName{
		Name: name,
	}
	user := userv1.User{}
	err := r.Get(context.Background(), key, &user)

	return user, err
}

func (r *DefaultResolver) ListUser() ([]userv1.User, error) {
	ul := userv1.UserList{}
	err := r.List(context.Background(), &ul)
	if err != nil {
		return nil, err
	}
	return ul.Items, err
}

func (r *DefaultResolver) GetRole(namespace, name string) (rbacv1.Role, error) {
	key := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	role := rbacv1.Role{}
	err := r.Get(context.Background(), key, &role)

	return role, err
}

func (r *DefaultResolver) GetClusterRole(name string) (rbacv1.ClusterRole, error) {
	key := types.NamespacedName{Name: name}
	clusterRole := rbacv1.ClusterRole{}
	err := r.Get(context.Background(), key, &clusterRole)

	return clusterRole, err
}

func (r *DefaultResolver) ListRoleBindings(namespace string) ([]rbacv1.RoleBinding, error) {
	rb := rbacv1.RoleBindingList{}
	err := r.List(context.Background(), &rb, &client.ListOptions{Namespace: namespace})
	if err != nil {
		return nil, err
	}
	return rb.Items, nil
}

func (r *DefaultResolver) ListClusterRoleBindings() ([]rbacv1.ClusterRoleBinding, error) {
	crb := rbacv1.ClusterRoleBindingList{}
	err := r.List(context.Background(), &crb)
	if err != nil {
		return nil, err
	}
	return crb.Items, err
}
