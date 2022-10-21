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

package hotplug

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	clusterv1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
	hotplugv1 "github.com/kubecube-io/kubecube/pkg/apis/hotplug/v1"
	"github.com/stretchr/testify/assert"
)

func clusterTemplate(name string) clusterv1.Cluster {
	return clusterv1.Cluster{
		TypeMeta:   metav1.TypeMeta{Kind: "Cluster", APIVersion: "cluster.kubecube.io/v1"},
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
}

func TestValidateDelete(t *testing.T) {
	assert := assert.New(t)
	// load scheme
	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)
	_ = v1.AddToScheme(scheme)

	// crete
	cluster1 := clusterTemplate("test-cluster1")
	cluster2 := clusterTemplate("common")
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(&cluster1, &cluster2).Build()

	// test delete if project still exist
	hotplugValidate := NewHotplugValidator(fakeClient)
	hotplugValidate.Name = "test-cluster1"
	err := hotplugValidate.ValidateCreate()
	assert.Nil(err)

	hotplugValidate.Name = "common"
	err = hotplugValidate.ValidateCreate()
	assert.Nil(err)

	hotplugValidate.Name = "test-cluster2"
	err = hotplugValidate.ValidateCreate()
	assert.NotNil(err)
	assert.Equal("the test-cluster2 not exist", err.Error())

	hotplugValidate.Name = "test-cluster1"
	hotplugValidate.Spec.Component = []hotplugv1.ComponentConfig{{Name: "abc"}, {Name: "def"}}
	err = hotplugValidate.ValidateCreate()
	assert.Nil(err)

	hotplugValidate.Name = "test-cluster1"
	hotplugValidate.Spec.Component = []hotplugv1.ComponentConfig{{Name: "abc"}, {Name: "abc"}}
	err = hotplugValidate.ValidateCreate()
	assert.NotNil(err)
	assert.Equal("the component name is repeat", err.Error())
}
