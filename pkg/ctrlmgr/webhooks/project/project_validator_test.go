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

func TestValidateCreate(t *testing.T) {
	assert := assert.New(t)

	// load scheme
	scheme := runtime.NewScheme()
	_ = tenantv1.AddToScheme(scheme)
	_ = v1.AddToScheme(scheme)

	// create
	tenant := tenantTemplate("test-tenant")
	fakeClient := fake.NewFakeClientWithScheme(scheme, &tenant)

	projectValidate := NewProjectValidator(fakeClient)

	// do not add label to project
	err := projectValidate.ValidateCreate()
	assert.NotNil(err)

	// add true label to project
	projectValidate.Labels = map[string]string{"kubecube.io/tenant": "test-tenant"}
	err = projectValidate.ValidateCreate()
	assert.Nil(err)

	// add tenat label, bug tenant no exist
	projectValidate.Labels = map[string]string{"kubecube.io/tenant": "test-tenant-no-exist"}
	err = projectValidate.ValidateCreate()
	assert.NotNil(err)

	// check false ingress domain suffix
	projectValidate.Labels = map[string]string{"kubecube.io/tenant": "test-tenant"}
	projectValidate.Spec.IngressDomainSuffix = []string{"test.com."}
	err = projectValidate.ValidateCreate()
	assert.NotNil(err)

	// check false ingress domain suffix
	projectValidate.Spec.IngressDomainSuffix = []string{"*^.com"}
	err = projectValidate.ValidateCreate()
	assert.NotNil(err)

	//check true ingress domain suffix
	projectValidate.Spec.IngressDomainSuffix = []string{".com"}
	err = projectValidate.ValidateCreate()
	assert.NotNil(err)

	//check true ingress domain suffix
	projectValidate.Spec.IngressDomainSuffix = []string{"test.com", "test", "com"}
	err = projectValidate.ValidateCreate()
	assert.Nil(err)
}

func TestValidateUpdate(t *testing.T) {
	assert := assert.New(t)

	// load scheme
	scheme := runtime.NewScheme()
	_ = tenantv1.AddToScheme(scheme)
	_ = v1.AddToScheme(scheme)

	// create
	tenant := tenantTemplate("test-tenant")
	fakeClient := fake.NewFakeClientWithScheme(scheme, &tenant)

	projectValidate := NewProjectValidator(fakeClient)

	// do not add label to project
	err := projectValidate.ValidateUpdate(nil)
	assert.NotNil(err)

	// add true label to project
	projectValidate.Labels = map[string]string{"kubecube.io/tenant": "test-tenant"}
	err = projectValidate.ValidateUpdate(nil)
	assert.Nil(err)

	// add tenat label, bug tenant no exist
	projectValidate.Labels = map[string]string{"kubecube.io/tenant": "test-tenant-no-exist"}
	err = projectValidate.ValidateUpdate(nil)
	assert.NotNil(err)

	// check false ingress domain suffix
	projectValidate.Labels = map[string]string{"kubecube.io/tenant": "test-tenant"}
	projectValidate.Spec.IngressDomainSuffix = []string{"test.com."}
	err = projectValidate.ValidateUpdate(nil)
	assert.NotNil(err)

	// check false ingress domain suffix
	projectValidate.Spec.IngressDomainSuffix = []string{"*^.com"}
	err = projectValidate.ValidateUpdate(nil)
	assert.NotNil(err)

	//check true ingress domain suffix
	projectValidate.Spec.IngressDomainSuffix = []string{".com"}
	err = projectValidate.ValidateUpdate(nil)
	assert.NotNil(err)

	//check true ingress domain suffix
	projectValidate.Spec.IngressDomainSuffix = []string{"test.com", "test", "com"}
	err = projectValidate.ValidateUpdate(nil)
	assert.Nil(err)
}

func TestValidateDelete(t *testing.T) {
	assert := assert.New(t)
	// load scheme
	scheme := runtime.NewScheme()
	_ = tenantv1.AddToScheme(scheme)
	_ = v1.AddToScheme(scheme)

	// create
	tenant := tenantTemplate("test-tenant")
	project1 := projectTemplate("test-tenant", "test-project1")
	project2 := projectTemplate("test-tenant", "test-project2")
	namespace := namespaceTemplate("test-project1", "test-namespace")
	fakeClient := fake.NewFakeClientWithScheme(scheme, &tenant, &project1, &project2, &namespace)

	// test delete if namespace still exist
	projectValidate := NewProjectValidator(fakeClient)
	projectValidate.Name = "test-project1"
	err := projectValidate.ValidateDelete()
	assert.NotNil(err)

	// test delete if namespace is empty
	projectValidate.Name = "test-project2"
	err = projectValidate.ValidateDelete()
	assert.Nil(err)
}
