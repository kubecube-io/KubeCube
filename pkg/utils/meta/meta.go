package meta

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

func TrimObjectMeta(obj runtime.Object) {
	objMeta, _ := meta.Accessor(obj)
	objMeta.SetFinalizers(nil)
	objMeta.SetDeletionGracePeriodSeconds(nil)
	objMeta.SetManagedFields(nil)
	objMeta.SetOwnerReferences(nil)
}
