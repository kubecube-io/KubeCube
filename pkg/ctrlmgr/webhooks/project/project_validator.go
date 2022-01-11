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
package project

import (
	"context"
	"fmt"
	tenantv1 "github.com/kubecube-io/kubecube/pkg/apis/tenant/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	projectLog    clog.CubeLogger
	projectClient client.Client
)

type ProjectValidator struct {
	tenantv1.Project
}

func NewProjectValidator(mgrClient client.Client) *ProjectValidator {
	projectLog = clog.WithName("Webhook").WithName("ProjectValidate")
	projectClient = mgrClient
	return &ProjectValidator{}
}

func (p *ProjectValidator) GetObjectKind() schema.ObjectKind {

	return p.GetObjectKind()
}

func (p *ProjectValidator) DeepCopyObject() runtime.Object {
	return &ProjectValidator{}
}

func (p *ProjectValidator) ValidateCreate() error {
	log := projectLog.WithValues("ValidateCreate", p.Name)

	tenantName := p.Labels["kubecube.io/tenant"]
	if tenantName == "" {
		log.Info("can not find .metadata.labels.kubecube.io/tenant label")
		return fmt.Errorf("can not find .metadata.labels.kubecube.io/tenant label")
	}

	ctx := context.Background()
	tenant := tenantv1.Tenant{}

	if err := projectClient.Get(ctx, types.NamespacedName{Name: tenantName}, &tenant); err != nil {
		log.Debug("The tenant %s is not exist", tenantName)
		return fmt.Errorf("the tenant is not exist")
	}

	if domainSuffixErr := validatorDomainSuffix(p, log); domainSuffixErr != nil {
		return domainSuffixErr
	}

	log.Debug("Create validate success")

	return nil
}

func (p *ProjectValidator) ValidateUpdate(old runtime.Object) error {
	log := projectLog.WithValues("ValidateUpdate", p.Name)

	tenantName := p.Labels["kubecube.io/tenant"]
	if tenantName == "" {
		log.Info("can not find .metadata.labels.kubecube.io/tenant label")
		return fmt.Errorf("can not find .metadata.labels.kubecube.io/tenant label")
	}

	ctx := context.Background()
	tenant := tenantv1.Tenant{}

	if err := projectClient.Get(ctx, types.NamespacedName{Name: tenantName}, &tenant); err != nil {
		log.Info("The tenant %s is not exist", tenantName)
		return fmt.Errorf("the tenant is not exist")
	}

	if domainSuffixErr := validatorDomainSuffix(p, log); domainSuffixErr != nil {
		return domainSuffixErr
	}

	log.Debug("Update validate success")

	return nil
}

func (p *ProjectValidator) ValidateDelete() error {
	log := projectLog.WithValues("ValidateDelete", p.Name)
	// 管辖的工作命名空间是否已经删除
	ctx := context.Background()
	namespaceList := v1.NamespaceList{}
	if err := projectClient.List(ctx, &namespaceList, client.MatchingLabels{"kubecube.io/project": p.Name}); err != nil {
		log.Error("Can not list namespaces under this project: %v", err.Error())
		return fmt.Errorf("can not list namespaces under this project")
	}
	if len(namespaceList.Items) > 0 {
		childResExistErr := fmt.Errorf("there are still namespaces under this project")
		log.Info("Delete fail: %v", childResExistErr.Error())
		return childResExistErr
	}

	// 关联的命名空间是否已经删除
	// 检查关联的命名空间是否已经删除
	ns := v1.Namespace{}
	err := projectClient.Get(ctx, types.NamespacedName{Name: p.Spec.Namespace}, &ns)
	if errors.IsNotFound(err) {
		log.Info("Delete validate success")
		return nil
	}

	return fmt.Errorf("the namespace %s is still exist", p.Spec.Namespace)
}

func validatorDomainSuffix(p *ProjectValidator, log clog.CubeLogger) error {
	domainSuffixList := p.Spec.IngressDomainSuffix
	if domainSuffixList != nil && len(domainSuffixList) != 0 {
		for _, domainSuffix := range domainSuffixList {
			errs := validation.IsDNS1123Subdomain(domainSuffix)
			if len(errs) > 0 {
				log.Debug("Invalid value: %s ", domainSuffix)
				return fmt.Errorf("Invalid value: %s : a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')\n)", domainSuffix)
			}
		}
	}
	return nil
}
