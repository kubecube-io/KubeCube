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

package tenant

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	tenantv1 "github.com/kubecube-io/kubecube/pkg/apis/tenant/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
)

func ValidateDelete(tenant *tenantv1.Tenant) error {
	ctx := context.Background()

	clusters := multicluster.Interface().FuzzyCopy()
	// check if exist related project
	projectList := tenantv1.ProjectList{}
	for _, cluster := range clusters {
		if err := cluster.Client.Cache().List(ctx, &projectList, client.MatchingLabels{constants.TenantLabel: tenant.Name}); err != nil {
			clog.Error("Can not list projects under this tenant: %v", err.Error())
			return fmt.Errorf("can not list projects under this tenant")
		}
		if len(projectList.Items) > 0 {
			childResExistErr := fmt.Errorf("there are still projects under this tenant")
			clog.Info("delete fail: %s", childResExistErr.Error())
			return childResExistErr
		}
	}
	return nil
}
