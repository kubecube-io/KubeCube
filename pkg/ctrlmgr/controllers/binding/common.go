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

	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
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
		if strings.HasPrefix(namespace, constants.TenantNsPrefix) {
			tenantMirror := strings.TrimPrefix(namespace, constants.TenantNsPrefix)
			if tenant != tenantMirror {
				return "", "", "", fmt.Errorf("RoleBinding (%s/%s) is not inconsistent with name and namespace", name, namespace)
			}
		}
	} else if len(projectInfos) == 2 {
		user, project = projectInfos[0], projectInfos[1]
		if strings.HasPrefix(namespace, constants.ProjectNsPrefix) {
			projectMirror := strings.TrimPrefix(namespace, constants.ProjectNsPrefix)
			if project != projectMirror {
				return "", "", "", fmt.Errorf("RoleBinding (%s/%s) is not inconsistent with name and namespace", name, namespace)
			}
		}
	}

	return user, tenant, project, nil
}

// parseUserInClusterRoleBinding will recognize ClusterRoleBinding which is related with user.
// The valid name format as follow:
// {user}-in-cluster
func parseUserInClusterRoleBinding(name string) (string, error) {
	if strings.HasPrefix(name, "gen-") {
		return "", nil
	}
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

func updateRoleBinding(ctx context.Context, cli client.Client, binding *rbacv1.RoleBinding) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		newBinding := &rbacv1.RoleBinding{}
		err := cli.Get(ctx, types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace}, newBinding)
		if err != nil {
			return err
		}

		newBinding.Labels = binding.Labels

		err = cli.Update(ctx, newBinding)
		if err != nil {
			return err
		}
		return nil
	})
}

func updateClusterRoleBinding(ctx context.Context, cli client.Client, binding *rbacv1.ClusterRoleBinding) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		newBinding := &rbacv1.ClusterRoleBinding{}
		err := cli.Get(ctx, types.NamespacedName{Name: binding.Name}, newBinding)
		if err != nil {
			return err
		}

		newBinding.Labels = binding.Labels

		err = cli.Update(ctx, newBinding)
		if err != nil {
			return err
		}
		return nil
	})
}

func setBindingUserLabel(labels map[string]string, user string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}

	labels[constants.LabelRelationship] = user

	return labels
}
