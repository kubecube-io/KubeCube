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

package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kubecube-io/kubecube/pkg/apis"
	clusterv1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
)

func clusterTemplate(name string) clusterv1.Cluster {
	return clusterv1.Cluster{
		TypeMeta:   metav1.TypeMeta{Kind: "Cluster", APIVersion: "cluster.kubecube.io/v1"},
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
}

func TestValidateCreate(t *testing.T) {
	assert := assert.New(t)

	// load scheme
	scheme := runtime.NewScheme()
	_ = apis.AddToScheme(scheme)

	// create
	cluster := clusterTemplate("test-cluster")
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(&cluster).Build()

	clusterValidate := NewClusterValidator(fakeClient)

	// check false ingress domain suffix
	clusterValidate.Spec.IngressDomainSuffix = "test.com."
	_, err := clusterValidate.ValidateCreate()
	assert.NotNil(err)

	// check false ingress domain suffix
	clusterValidate.Spec.IngressDomainSuffix = "*^.com"
	_, err = clusterValidate.ValidateCreate()
	assert.NotNil(err)

	//check true ingress domain suffix
	clusterValidate.Spec.IngressDomainSuffix = ".com"
	_, err = clusterValidate.ValidateCreate()
	assert.NotNil(err)

	//check true ingress domain suffix
	clusterValidate.Spec.IngressDomainSuffix = "test.com"
	_, err = clusterValidate.ValidateCreate()
	assert.Nil(err)
}

func TestValidateUpdate(t *testing.T) {
	assert := assert.New(t)

	// load scheme
	scheme := runtime.NewScheme()
	_ = apis.AddToScheme(scheme)

	// create
	cluster := clusterTemplate("test-cluster")
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(&cluster).Build()

	clusterValidate := NewClusterValidator(fakeClient)

	// check false ingress domain suffix
	clusterValidate.Spec.IngressDomainSuffix = "test.com."
	_, err := clusterValidate.ValidateUpdate(nil)
	assert.NotNil(err)

	// check false ingress domain suffix
	clusterValidate.Spec.IngressDomainSuffix = "*^.com"
	_, err = clusterValidate.ValidateUpdate(nil)
	assert.NotNil(err)

	//check true ingress domain suffix
	clusterValidate.Spec.IngressDomainSuffix = ".com"
	_, err = clusterValidate.ValidateUpdate(nil)
	assert.NotNil(err)

	//check true ingress domain suffix
	clusterValidate.Spec.IngressDomainSuffix = "test.com"
	_, err = clusterValidate.ValidateUpdate(nil)
	assert.Nil(err)
}
