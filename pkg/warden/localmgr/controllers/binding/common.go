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
	"fmt"
	"strings"

	v1 "github.com/kubecube-io/kubecube/pkg/apis/user/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// parseUserInRoleBinding will recognize RoleBinding under tenant or project namespace.
// The valid name and namespace format as follow:
// {user}-in-kubecube-tenant-{tenantName}/kubecube-tenant-{tenantName}
// {user}-in-kubecube-project-{projectName}/kubecube-project-{projectName}
//
// Note: It must match namespace nad name suffix which means RoleBindings spread by hnc
// will be ignored.
func parseUserInRoleBinding(name, namespace string) (user string, tenant string, project string, err error) {
	tenantInfos := strings.Split(name, "-in-kubecube-tenant-")
	projectInfos := strings.Split(name, "-in-kubecube-project-")

	if len(tenantInfos) != 2 && len(projectInfos) != 2 {
		return "", "", "", fmt.Errorf("parse user in RoleBinding (%s/%s) failed", name, namespace)
	}

	if len(tenantInfos) == 2 {
		user, tenant = tenantInfos[0], tenantInfos[1]
		if !strings.HasPrefix(namespace, constants.TenantNsPrefix) {
			return "", "", "", nil
		}
		tenantMirror := strings.TrimPrefix(namespace, constants.TenantNsPrefix)
		if tenant != tenantMirror {
			return "", "", "", fmt.Errorf("RoleBinding (%s/%s) is not inconsistent with name and namespace", name, namespace)
		}
	} else if len(projectInfos) == 2 {
		user, project = projectInfos[0], projectInfos[1]
		if !strings.HasPrefix(namespace, constants.ProjectNsPrefix) {
			return "", "", "", nil
		}
		projectMirror := strings.TrimPrefix(namespace, constants.ProjectNsPrefix)
		if project != projectMirror {
			return "", "", "", fmt.Errorf("RoleBinding (%s/%s) is not inconsistent with name and namespace", name, namespace)
		}
	}

	return user, tenant, project, nil
}

// parseUserInClusterRoleBinding will recognize ClusterRoleBinding which is related with user.
// The valid name format as follow:
// {user}-in-cluster
func parseUserInClusterRoleBinding(name string) (string, error) {
	if !strings.HasSuffix(name, "-in-cluster") {
		return "", fmt.Errorf("parse user in ClusterRoleBinding %s failed", name)
	}
	return strings.TrimSuffix(name, "-in-cluster"), nil
}

// parseUserNameWithID try to parse username with id.
// the valid name format follow:
// {user}-{id}
func parseUserNameWithID(name string) string {
	return strings.Split(name, "-")[0]
}

func addUserToTenant(user *v1.User, tenant string) {
	tenantSet := sets.NewString(user.Status.BelongTenants...)
	tenantSet.Insert(tenant)
	user.Status.BelongTenants = tenantSet.List()
	clog.Info("ensure user %v belongs to tenant %v", user.Name, tenant)
}

func addUserToProject(user *v1.User, project string) {
	projectSet := sets.NewString(user.Status.BelongProjects...)
	projectSet.Insert(project)
	user.Status.BelongProjects = projectSet.List()
	clog.Info("ensure user %v belongs to project %v", user.Name, project)
}

func moveUserFromTenant(user *v1.User, tenant string) {
	tenantSet := sets.NewString(user.Status.BelongTenants...)
	tenantSet.Delete(tenant)
	user.Status.BelongTenants = tenantSet.List()
	clog.Info("move tenant %v of user %v", tenant, user.Name)
}

func moveUserFromProject(user *v1.User, project string) {
	projectSet := sets.NewString(user.Status.BelongProjects...)
	projectSet.Delete(project)
	user.Status.BelongProjects = projectSet.List()
	clog.Info("move project %v of user %v", user.Name, project)
}

func appointUserAdmin(user *v1.User) {
	user.Status.PlatformAdmin = true
	clog.Info("appoint user %v is platform admin", user.Name)
}

func impeachUserAdmin(user *v1.User) {
	user.Status.PlatformAdmin = false
	clog.Info("impeach platform admin of user %v", user.Name)
}

func updateUserStatus(ctx context.Context, cli client.Client, user *v1.User, scope string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		newUser := &v1.User{}
		err := cli.Get(ctx, types.NamespacedName{Name: user.Name}, newUser)
		if err != nil {
			return err
		}

		switch scope {
		case constants.ClusterRolePlatform:
			newUser.Status.PlatformAdmin = user.Status.PlatformAdmin
		case constants.ClusterRoleTenant:
			newUser.Status.BelongTenants = user.Status.BelongTenants
		case constants.ClusterRoleProject:
			newUser.Status.BelongProjects = user.Status.BelongProjects
		}

		err = cli.Status().Update(ctx, newUser)
		if err != nil {
			return err
		}
		return nil
	})
}
