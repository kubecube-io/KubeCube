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

package authorization

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	userinfo "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tenantv1 "github.com/kubecube-io/kubecube/pkg/apis/tenant/v1"
	userv1 "github.com/kubecube-io/kubecube/pkg/apis/user/v1"
	"github.com/kubecube-io/kubecube/pkg/authorizer/mapping"
	"github.com/kubecube-io/kubecube/pkg/authorizer/rbac"
	"github.com/kubecube-io/kubecube/pkg/clog"
	mgrclient "github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	rbacv1 "k8s.io/api/rbac/v1"
)

func makeRoleNamesByPtr(roles []*rbacv1.Role, clusterRoles []*rbacv1.ClusterRole) ([]string, []string) {
	roleNames := make([]string, 0, len(roles))
	clusterRoleNames := make([]string, 0, len(clusterRoles))
	for _, r := range roles {
		roleNames = append(roleNames, r.Name)
	}
	for _, r := range clusterRoles {
		clusterRoleNames = append(clusterRoleNames, r.Name)
	}
	return roleNames, clusterRoleNames
}

// todo: to optimize
func makeRoleNames(roles []rbacv1.Role, clusterRoles []rbacv1.ClusterRole) ([]string, []string) {
	roleNames := make([]string, 0, len(roles))
	clusterRoleNames := make([]string, 0, len(clusterRoles))
	for _, r := range roles {
		roleNames = append(roleNames, r.Name)
	}
	for _, r := range clusterRoles {
		clusterRoleNames = append(clusterRoleNames, r.Name)
	}
	return roleNames, clusterRoleNames
}

func makeUserNames(users []*userv1.User) []string {
	userNames := make([]string, 0, len(users))

	for _, u := range users {
		userNames = append(userNames, u.Name)
	}

	return userNames
}

// getAccessTenants get visible tenants of user
func getAccessTenants(rbac rbac.Interface, user string, cli mgrclient.Client, ctx context.Context, auth string) (result, error) {
	tenantSet := sets.NewString()
	tenantList := tenantv1.TenantList{}
	err := cli.Cache().List(ctx, &tenantList)
	if err != nil {
		return result{}, err
	}

	for _, t := range tenantList.Items {
		if isAllowedAccessNs(rbac, user, t.Spec.Namespace, auth) {
			tenantSet.Insert(t.Name)
		}
	}

	projectList := tenantv1.ProjectList{}
	err = cli.Cache().List(ctx, &projectList)
	if err != nil {
		return result{}, err
	}

	// get tenant of project if user is project scope
	for _, p := range projectList.Items {
		if isAllowedAccessNs(rbac, user, p.Spec.Namespace, auth) {
			t, ok := p.Labels[constants.TenantLabel]
			if !ok {
				continue
			}
			tenantSet.Insert(t)
		}
	}

	tenants := tenantSet.List()
	lists := make([]tenantv1.Tenant, 0)
	for _, t := range tenants {
		tenant := tenantv1.Tenant{}
		key := types.NamespacedName{Name: t}
		err = cli.Cache().Get(ctx, key, &tenant)
		if err != nil {
			clog.Warn(err.Error())
			continue
		}
		lists = append(lists, tenant)
	}

	res := result{
		Total: len(lists),
		Items: lists,
	}

	return res, nil
}

// getAccessProjects get visible projects of user
func getAccessProjects(rbac rbac.Interface, user string, cli mgrclient.Client, ctx context.Context, tenant []string, auth string) (result, error) {
	var lists []tenantv1.Project
	projectList := tenantv1.ProjectList{}
	err := cli.Cache().List(ctx, &projectList)
	if err != nil {
		return result{}, err
	}

	for _, p := range projectList.Items {
		if isAllowedAccessNs(rbac, user, p.Spec.Namespace, auth) {
			lists = append(lists, p)
		}
	}

	if len(tenant) != 0 {
		lists = filterBy(tenant, lists)
	}
	res := result{
		Total: len(lists),
		Items: lists,
	}

	return res, nil
}

func filterBy(tenant []string, projects []tenantv1.Project) (res []tenantv1.Project) {
	tenantSet := sets.NewString(tenant...)
	for _, p := range projects {
		t, ok := p.Labels[constants.TenantLabel]
		if !ok {
			continue
		}
		if _, ok := tenantSet[t]; ok {
			res = append(res, p)
		}
	}
	return
}

// isAllowedAccessNs consider user has visible view of given namespace
// if user can get pods in that namespace.
func isAllowedAccessNs(rbac rbac.Interface, user, namespace, auth string) bool {
	verb := constants.GetVerb
	if auth == constants.Writable {
		verb = constants.CreateVerb
	}

	a := authorizer.AttributesRecord{
		User:            &userinfo.DefaultInfo{Name: user},
		Verb:            verb,
		Namespace:       namespace,
		Resource:        constants.K8sResourcePod,
		ResourceRequest: true,
	}
	d, _, err := rbac.Authorize(context.Background(), a)
	if err != nil {
		clog.Error(err.Error())
	}

	return d == authorizer.DecisionAllow
}

// getAllRoles get all roles and cluster roles related with kubecube
func getAllRoles(ctx context.Context, cli mgrclient.Client) (map[string]interface{}, error) {
	labelSelector, err := labels.Parse(fmt.Sprintf("%v=%v", constants.RbacLabel, true))
	if err != nil {
		return nil, err
	}

	clusterRoleList := rbacv1.ClusterRoleList{}
	err = cli.Cache().List(ctx, &clusterRoleList, &client.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, err
	}

	roleList := rbacv1.RoleList{}
	err = cli.Cache().List(ctx, &roleList, &client.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, err
	}

	roleNames, clusterRoleNames := makeRoleNames(roleList.Items, clusterRoleList.Items)

	r := make(map[string]interface{})
	r["roles"] = result{
		Total: len(roleNames),
		Items: roleNames,
	}
	r["clusterRoles"] = result{
		Total: len(clusterRoleNames),
		Items: clusterRoleNames,
	}

	return r, nil
}

// getRolesByNs get all role under tenant or project
func getRolesByNs(ctx context.Context, cli mgrclient.Client, ns string) (map[string]interface{}, error) {
	const (
		symbol        = "-"
		tenantPrefix  = "kubecube-tenant"
		projectPrefix = "kubecube-project"
	)

	strs := strings.Split(ns, "-")
	if len(strs) < 3 {
		return nil, fmt.Errorf("unknown namespace format: %v", ns)
	}

	// listClusterRoleFn list ClusterRole by given label selectors
	listClusterRoleFn := func(labelStr string) ([]string, error) {
		labelSelector, err := labels.Parse(labelStr)
		if err != nil {
			return nil, err
		}

		list := rbacv1.ClusterRoleList{}
		err = cli.Cache().List(ctx, &list, &client.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			return nil, err
		}
		res := make([]string, 0, len(list.Items))
		for _, v := range list.Items {
			res = append(res, v.Name)
		}
		return res, nil
	}

	r := make(map[string]interface{})

	prefix := strs[0] + symbol + strs[1]
	switch prefix {
	case tenantPrefix:
		res, err := listClusterRoleFn(fmt.Sprintf("%v=%v", constants.RoleLabel, "tenant"))
		if err != nil {
			return nil, err
		}
		res = append(res, constants.Reviewer)
		r["clusterRoles"] = result{Total: len(res), Items: res}
	case projectPrefix:
		res, err := listClusterRoleFn(fmt.Sprintf("%v=%v", constants.RoleLabel, "project"))
		if err != nil {
			return nil, err
		}
		res = append(res, constants.Reviewer)
		r["clusterRoles"] = result{Total: len(res), Items: res}
	default:
		return nil, fmt.Errorf("unknown prefix of namespace: %v", prefix)
	}

	labelSelector, err := labels.Parse(fmt.Sprintf("%v=%v", constants.RbacLabel, true))
	if err != nil {
		return nil, err
	}

	roleList := rbacv1.RoleList{}
	err = cli.Cache().List(ctx, &roleList, &client.ListOptions{LabelSelector: labelSelector, Namespace: ns})
	if err != nil {
		return nil, err
	}

	roleNames, _ := makeRoleNames(roleList.Items, nil)

	r["roles"] = result{
		Total: len(roleNames),
		Items: roleNames,
	}

	return r, nil
}

func isPlatformAdmin(r rbac.Interface, user string) bool {
	_, clusterRoles, err := r.RolesFor(rbac.User2UserInfo(user), "")
	if err != nil {
		clog.Error(err.Error())
		return false
	}
	for _, clusterRole := range clusterRoles {
		if clusterRole.Name == constants.PlatformAdmin {
			return true
		}
	}
	return false
}

func isTenantAdmin(r rbac.Interface, cli mgrclient.Client, user string) bool {
	tenantList := tenantv1.TenantList{}
	err := cli.Cache().List(context.Background(), &tenantList)
	if err != nil {
		clog.Error(err.Error())
		return false
	}

	for _, t := range tenantList.Items {
		ns := t.Spec.Namespace
		_, clusterRoles, err := r.RolesFor(rbac.User2UserInfo(user), ns)
		if err != nil {
			clog.Warn(err.Error())
			continue
		}
		for _, r := range clusterRoles {
			if r.Name == constants.TenantAdmin {
				return true
			}
		}
	}

	return false
}

func isProjectAdmin(r rbac.Interface, cli mgrclient.Client, user string) bool {
	projectList := tenantv1.ProjectList{}
	err := cli.Cache().List(context.Background(), &projectList)
	if err != nil {
		clog.Error(err.Error())
		return false
	}

	for _, p := range projectList.Items {
		ns := p.Spec.Namespace
		_, clusterRoles, err := r.RolesFor(rbac.User2UserInfo(user), ns)
		if err != nil {
			clog.Warn(err.Error())
			continue
		}
		for _, r := range clusterRoles {
			if r.Name == constants.ProjectAdmin {
				return true
			}
		}
	}

	return false
}

func isAllowedAccess(rbac rbac.Interface, user, resource, namespace, auth string) bool {
	read, write, res1, res2 := false, false, true, true

	switch auth {
	case mapping.Read:
		read = true
	case mapping.Write:
		write = true
	case mapping.Both:
		read, write = true, true
	}

	// note:we just sort up auth to write and read, take care of it
	if read {
		a := authorizer.AttributesRecord{
			User:            &userinfo.DefaultInfo{Name: user},
			Verb:            "get",
			Namespace:       namespace,
			Resource:        resource,
			ResourceRequest: true,
		}
		d, _, err := rbac.Authorize(context.Background(), a)
		if err != nil {
			clog.Error(err.Error())
		}
		res1 = d == authorizer.DecisionAllow
	}

	if write {
		a := authorizer.AttributesRecord{
			User:            &userinfo.DefaultInfo{Name: user},
			Verb:            "create",
			Namespace:       namespace,
			Resource:        resource,
			ResourceRequest: true,
		}
		d, _, err := rbac.Authorize(context.Background(), a)
		if err != nil {
			clog.Error(err.Error())
		}
		res2 = d == authorizer.DecisionAllow
	}

	return res1 && res2
}
