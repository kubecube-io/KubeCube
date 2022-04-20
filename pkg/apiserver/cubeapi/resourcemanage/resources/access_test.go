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
