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

package controllers

import (
	"context"
	"testing"

	tenantv1 "github.com/kubecube-io/kubecube/pkg/apis/tenant/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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

func TestReconcile(t *testing.T) {
	assert := assert.New(t)
	// load scheme
	scheme := runtime.NewScheme()
	_ = tenantv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	// crete
	tenant1 := tenantTemplate("test-tenant1")
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(&tenant1).Build()

	tenantReconciler := TenantReconciler{}
	tenantReconciler.Client = fakeClient
	tenantReconciler.Scheme = scheme

	req := ctrl.Request{}
	req.Name = "test-tenant1"
	req.NamespacedName = types.NamespacedName{Name: req.Name}

	ctx := context.Background()
	_, err := tenantReconciler.Reconcile(ctx, req)
	assert.Nil(err)

	namespace := corev1.Namespace{}
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "kubecube-tenant-test-tenant1"}, &namespace)
	assert.Nil(err)

}
