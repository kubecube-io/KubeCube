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
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	tenantv1 "github.com/kubecube-io/kubecube/pkg/apis/tenant/v1"
	"github.com/stretchr/testify/assert"
)

// tenant template
func tenantTemplate(name string) tenantv1.Tenant {
	return tenantv1.Tenant{
		TypeMeta:   metav1.TypeMeta{Kind: "tenant", APIVersion: "tenant.kubecube.io/v1"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "", Name: name},
		Spec: tenantv1.TenantSpec{
			DisplayName: "test-tenant",
			Description: "test tenant",
		},
	}
}

// project template
func projectTemplate(tenant string, name string) tenantv1.Project {
	return tenantv1.Project{
		TypeMeta:   metav1.TypeMeta{Kind: "project", APIVersion: "tenant.kubecube.io/v1"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "", Name: name, Labels: map[string]string{"kubecube.io/tenant": tenant}},
		Spec: tenantv1.ProjectSpec{
			DisplayName: "test-project",
			Description: "test project",
		},
	}
}

// namespace template
func namespaceTemplate(project string, name string) v1.Namespace {
	return v1.Namespace{
		TypeMeta:   metav1.TypeMeta{Kind: "Namespace", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "", Name: name, Labels: map[string]string{"kubecube.io/project": project}},
	}
}

func TestValidateDelete(t *testing.T) {
	assert := assert.New(t)
	// load scheme
	scheme := runtime.NewScheme()
	_ = tenantv1.AddToScheme(scheme)
	_ = v1.AddToScheme(scheme)

	// crete
	tenant1 := tenantTemplate("test-tenant1")
	tenant2 := tenantTemplate("test-tenant2")
	project := projectTemplate("test-tenant1", "test-project")
	namespace := namespaceTemplate("test-project1", "test-namespace")
	fakeClient := fake.NewFakeClientWithScheme(scheme, &tenant1, &tenant2, &project, &namespace)

	// test delete if project still exist
	tenantValidate := NewTenantValidator(fakeClient)
	tenantValidate.Name = "test-tenant1"
	err := tenantValidate.ValidateDelete()
	assert.NotNil(err)
	assert.Equal("there are still projects under this tenant", err.Error())

	// test delete if project is empty
	tenantValidate.Name = "test-tenant2"
	err = tenantValidate.ValidateDelete()
	assert.Nil(err)
}
