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
package tenant

import (
	"context"
	"fmt"

	tenantv1 "github.com/kubecube-io/kubecube/pkg/apis/tenant/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
)

var (
	tenantLog clog.CubeLogger

	tenantClient client.Client
)

type TenantValidator struct {
	tenantv1.Tenant
}

func NewTenantValidator(mgrClient client.Client) *TenantValidator {
	tenantLog = clog.WithName("Webhook").WithName("TenantValidate")
	tenantClient = mgrClient
	return &TenantValidator{}
}

func (t *TenantValidator) GetObjectKind() schema.ObjectKind {

	return t.GetObjectKind()
}

func (t *TenantValidator) DeepCopyObject() runtime.Object {
	return &TenantValidator{}
}

func (t *TenantValidator) ValidateCreate() error {

	return nil
}

func (t *TenantValidator) ValidateUpdate(old runtime.Object) error {

	return nil
}

func (t *TenantValidator) ValidateDelete() error {
	log := tenantLog.WithValues("ValidateDelete", t.Name)
	ctx := context.Background()

	// 通过标签检查是否还有项目关联
	projectList := tenantv1.ProjectList{}
	if err := tenantClient.List(ctx, &projectList, client.MatchingLabels{"kubecube.io/tenant": t.Name}); err != nil {
		log.Error("Can not list projects under this tenant: %v", err.Error())
		return fmt.Errorf("can not list projects under this tenant")
	}
	if len(projectList.Items) > 0 {
		childResExistErr := fmt.Errorf("there are still projects under this tenant")
		log.Info("delete fail: %s", childResExistErr.Error())
		return childResExistErr
	}

	// 检查关联的命名空间是否已经删除
	ns := corev1.Namespace{}
	err := tenantClient.Get(ctx, types.NamespacedName{Name: t.Spec.Namespace}, &ns)
	if errors.IsNotFound(err) {
		return nil
	}

	return fmt.Errorf("the namespace %s is still exist", t.Spec.Namespace)
}
