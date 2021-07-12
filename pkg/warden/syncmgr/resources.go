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

package syncmgr

import (
	"fmt"

	hotplugv1 "github.com/kubecube-io/kubecube/pkg/apis/hotplug/v1"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/multi-tenancy/incubator/hnc/api/v1alpha2"

	v1 "k8s.io/api/rbac/v1"

	cluster "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
	quota "github.com/kubecube-io/kubecube/pkg/apis/quota/v1"
	tenant "github.com/kubecube-io/kubecube/pkg/apis/tenant/v1"
	user "github.com/kubecube-io/kubecube/pkg/apis/user/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// syncResources define resources need be sync
var syncResources = []client.Object{
	&v1.RoleBinding{},
	&v1.ClusterRoleBinding{},
	&v1.Role{},
	&v1.ClusterRole{},

	&corev1.Namespace{},
	&v1alpha2.SubnamespaceAnchor{},

	&hotplugv1.Hotplug{},
	&tenant.Tenant{},
	&tenant.Project{},
	&user.User{},
	//&cluster.Cluster{},
	&quota.CubeResourceQuota{},
}

// newGenericObj new a struct point implemented client.Object
func newGenericObj(obj client.Object) (client.Object, error) {
	switch obj.(type) {
	case *v1.Role:
		return &v1.Role{}, nil
	case *v1.RoleBinding, nil:
		return &v1.RoleBinding{}, nil
	case *v1.ClusterRole:
		return &v1.ClusterRole{}, nil
	case *v1.ClusterRoleBinding:
		return &v1.ClusterRoleBinding{}, nil
	case *user.User:
		return &user.User{}, nil
	case *cluster.Cluster:
		return &cluster.Cluster{}, nil
	case *tenant.Project:
		return &tenant.Project{}, nil
	case *tenant.Tenant:
		return &tenant.Tenant{}, nil
	case *quota.CubeResourceQuota:
		return &quota.CubeResourceQuota{}, nil
	case *corev1.Namespace:
		return &corev1.Namespace{}, nil
	case *v1alpha2.SubnamespaceAnchor:
		return &v1alpha2.SubnamespaceAnchor{}, nil
	case *hotplugv1.Hotplug:
		return &hotplugv1.Hotplug{}, nil
	default:
		return nil, fmt.Errorf("unsupport sync resource: %v", obj.GetObjectKind().GroupVersionKind().String())
	}
}
