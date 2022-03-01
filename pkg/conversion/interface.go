package conversion

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type SingleVersionConverter interface {
	Convert(in runtime.Object, target runtime.GroupVersioner) (runtime.Object, error)

	Encode(obj runtime.Object, gv runtime.GroupVersioner) ([]byte, error)
	Decode(data []byte, defaults *schema.GroupVersionKind, into runtime.Object, versions ...schema.GroupVersion) (runtime.Object, *schema.GroupVersionKind, error)

	IsObjectAvailable(obj runtime.Object) (bool, *schema.GroupVersionKind, error)
	IsGvrAvailable(gvr *schema.GroupVersionResource) (bool, *schema.GroupVersionKind, error)
	IsGvkAvailable(gvk *schema.GroupVersionKind) (bool, *schema.GroupVersionKind, error)
}

type MultiVersionConverter interface {
	GetVersionConvert(cluster string) (*VersionConverter, error)
}
