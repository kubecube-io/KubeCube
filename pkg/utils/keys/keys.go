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

package keys

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
)

// ClusterWideKey is the object key which is a unique identifier under a cluster, across all resources.
type ClusterWideKey struct {
	// Group is the API Group of resource being referenced.
	Group string

	// Version is the API Version of the resource being referenced.
	Version string

	// Kind is the type of resource being referenced.
	Kind string

	// Namespace is the name of a namespace.
	Namespace string

	// Name is the name of resource being referenced.
	Name string
}

// String returns the key's printable info with format:
// "<GroupVersion>, kind=<Kind>, <NamespaceKey>"
func (k ClusterWideKey) String() string {
	return fmt.Sprintf("%s, kind=%s, %s", k.GroupVersion().String(), k.Kind, k.NamespaceKey())
}

// NamespaceKey returns the traditional key of a object.
func (k *ClusterWideKey) NamespaceKey() string {
	if len(k.Namespace) > 0 {
		return k.Namespace + "/" + k.Name
	}

	return k.Name
}

// GroupVersionKind returns the group, version, and kind of resource being referenced.
func (k *ClusterWideKey) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   k.Group,
		Version: k.Version,
		Kind:    k.Kind,
	}
}

// GroupVersion returns the group and version of resource being referenced.
func (k *ClusterWideKey) GroupVersion() schema.GroupVersion {
	return schema.GroupVersion{
		Group:   k.Group,
		Version: k.Version,
	}
}

// ClusterWideKeyFunc generates a ClusterWideKey for object.
func ClusterWideKeyFunc(obj interface{}) (ClusterWideKey, error) {
	key := ClusterWideKey{}

	runtimeObject, ok := obj.(runtime.Object)
	if !ok {
		klog.Errorf("Invalid object")
		return key, fmt.Errorf("not runtime object")
	}

	metaInfo, err := meta.Accessor(obj)
	if err != nil { // should not happen
		return key, fmt.Errorf("object has no meta: %v", err)
	}

	gvk := runtimeObject.GetObjectKind().GroupVersionKind()
	key.Group = gvk.Group
	key.Version = gvk.Version
	key.Kind = gvk.Kind
	key.Namespace = metaInfo.GetNamespace()
	key.Name = metaInfo.GetName()

	return key, nil
}
