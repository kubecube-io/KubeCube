package conversion

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SingleVersionConverter interface {
	// Convert converts an Object to another, generally the conversion is internalVersion <-> versioned.
	// if out was set, the converted result would be set into.
	Convert(in runtime.Object, out runtime.Object, target runtime.GroupVersioner) (runtime.Object, error)
	// DirectConvert converts a versioned Object to another version with given target gv.
	// if out was set, the converted result would be set into.
	DirectConvert(in runtime.Object, out runtime.Object, target runtime.GroupVersioner) (runtime.Object, error)

	// Encode encodes given obj, generally the gv should match Object
	Encode(obj runtime.Object, gv runtime.GroupVersioner) ([]byte, error)
	// Decode decodes data to object, if defaults was not set, the internalVersion would be used.
	Decode(data []byte, defaults *schema.GroupVersionKind, into runtime.Object, versions ...schema.GroupVersion) (runtime.Object, *schema.GroupVersionKind, error)

	// IsObjectAvailable describes if given object is available in target cluster.
	// a recommend group version kind will return if it cloud not pass through.
	IsObjectAvailable(obj runtime.Object) (isPassThrough bool, rawGvk *schema.GroupVersionKind, recommendGvk *schema.GroupVersionKind, err error)
	// IsGvrAvailable describes if given gvr is available in target cluster.
	// a recommend group version kind will return if it cloud not pass through.
	IsGvrAvailable(gvr *schema.GroupVersionResource) (isPassThrough bool, rawGvk *schema.GroupVersionKind, recommendGvk *schema.GroupVersionKind, err error)
	// IsGvkAvailable describes if given gvk is available in target cluster.
	// a recommend group version kind will return if it cloud not pass through.
	IsGvkAvailable(gvk *schema.GroupVersionKind) (isPassThrough bool, rawGvk *schema.GroupVersionKind, recommendGvk *schema.GroupVersionKind, err error)
}

// ReaderWithConverter wrap a Reader with given VersionConverter
type ReaderWithConverter interface {
	SingleVersionConverter
	client.Reader
}

// WriterWithConverter wrap a Writer with given VersionConverter
type WriterWithConverter interface {
	SingleVersionConverter
	client.Writer
}

// StatusWriterWithConverter wrap a StatusWriter with given VersionConverter
type StatusWriterWithConverter interface {
	SingleVersionConverter
	client.StatusWriter
}

// MultiVersionConverter holds multi VersionConverters by different cluster name
type MultiVersionConverter interface {
	GetVersionConvert(cluster string) (*VersionConverter, error)
}
