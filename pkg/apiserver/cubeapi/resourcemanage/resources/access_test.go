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

package resources

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	"github.com/kubecube-io/kubecube/pkg/multicluster/client/fake"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/stretchr/testify/assert"
)

func TestAccessAllow(t *testing.T) {
	scheme := runtime.NewScheme()
	opts := &fake.Options{
		Scheme:               scheme,
		Objs:                 []client.Object{},
		ClientSetRuntimeObjs: []runtime.Object{},
		Lists:                []client.ObjectList{},
	}
	multicluster.InitFakeMultiClusterMgrWithOpts(opts)
	clients.InitCubeClientSetWithOpts(nil)

	assert := assert.New(t)
	access := NewSimpleAccess(constants.LocalCluster, "admin", "namespace-test")
	allow := access.AccessAllow("", "pods", "list")
	assert.Equal(allow, false)
}
