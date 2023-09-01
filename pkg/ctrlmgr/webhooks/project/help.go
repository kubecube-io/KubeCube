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

package project

import (
	"context"
	"errors"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	tenantv1 "github.com/kubecube-io/kubecube/pkg/apis/tenant/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/domain"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var notFoundLabelErr = errors.New("can not find .metadata.labels.kubecube.io/tenant label")

func (r *Validator) ValidateCreate(project *tenantv1.Project) error {
	tenantName := project.Labels[constants.TenantLabel]
	if tenantName == "" {
		clog.Info(notFoundLabelErr.Error())
		return notFoundLabelErr
	}

	ctx := context.Background()
	tenant := tenantv1.Tenant{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: tenantName}, &tenant); err != nil {
		clog.Info("The tenant %s is not exist", tenantName)
		return fmt.Errorf("the tenant is not exist")
	}

	if err := domain.ValidatorDomainSuffix(project.Spec.IngressDomainSuffix); err != nil {
		return err
	}

	clog.Debug("Create validate success, project info: %v", project)
	return nil
}

func (r *Validator) ValidateUpdate(_ *tenantv1.Project, currentProject *tenantv1.Project) error {

	tenantName := currentProject.Labels[constants.TenantLabel]
	if tenantName == "" {
		clog.Info(notFoundLabelErr.Error())
		return notFoundLabelErr
	}

	ctx := context.Background()
	tenant := tenantv1.Tenant{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: tenantName}, &tenant); err != nil {
		clog.Info("The tenant %s is not exist", tenantName)
		return fmt.Errorf("the tenant is not exist")
	}

	if err := domain.ValidatorDomainSuffix(currentProject.Spec.IngressDomainSuffix); err != nil {
		return err
	}

	clog.Debug("Update validate success, project info: %v", currentProject)

	return nil
}

func (r *Validator) ValidateDelete(project *tenantv1.Project) error {
	// check the namespace we take over has been already deleted
	ctx := context.Background()
	clusters := multicluster.Interface().FuzzyCopy()

	lbSelector, err := labels.Parse(fmt.Sprintf("%v%v.tree.hnc.x-k8s.io/depth=1", constants.ProjectNsPrefix, project.Name))
	if err != nil {
		return err
	}

	for _, cluster := range clusters {
		namespaceList := v1.NamespaceList{}
		if err := cluster.Client.Cache().List(ctx, &namespaceList, &client.ListOptions{LabelSelector: lbSelector}); err != nil {
			clog.Error("Can not list namespaces under this project: %v", err.Error())
			return fmt.Errorf("can not list namespaces under this project")
		}
		if len(namespaceList.Items) > 0 {
			childResExistErr := fmt.Errorf("there are still namespaces under this project")
			clog.Info("Delete fail: %v", childResExistErr.Error())
			return childResExistErr
		}
	}
	return nil
}
