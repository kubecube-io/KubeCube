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

package conversion

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type wrapperWriter struct {
	// convertBack the gate to control if there is necessary
	// to convert action result back to user
	convertBack bool

	client.Writer
	SingleVersionConverter
}

var _ WriterWithConverter = &wrapperWriter{}

func WrapWriter(w client.Writer, c SingleVersionConverter, convertBack bool) WriterWithConverter {
	return &wrapperWriter{
		convertBack:            convertBack,
		Writer:                 w,
		SingleVersionConverter: c,
	}
}

// Create saves the object obj in the Kubernetes cluster.
func (w *wrapperWriter) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	converted, rawGvk, isPassThrough, err := tryConvert(obj, w.SingleVersionConverter)
	if err != nil {
		return err
	}
	if isPassThrough {
		return w.Writer.Create(ctx, obj, opts...)
	}
	if err = w.Writer.Create(ctx, converted, opts...); err != nil {
		return err
	}
	if w.convertBack {
		if err = tryConvertBack(converted, obj, rawGvk, w.SingleVersionConverter); err != nil {
			return err
		}
	}
	return nil
}

// Delete deletes the given obj from Kubernetes cluster.
func (w *wrapperWriter) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	converted, _, isPassThrough, err := tryConvert(obj, w.SingleVersionConverter)
	if err != nil {
		return err
	}
	if isPassThrough {
		return w.Writer.Delete(ctx, obj, opts...)
	}
	return w.Writer.Delete(ctx, converted, opts...)
}

// Update updates the given obj in the Kubernetes cluster. obj must be a
// struct pointer so that obj can be updated with the content returned by the Server.
func (w *wrapperWriter) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	converted, rawGvk, isPassThrough, err := tryConvert(obj, w.SingleVersionConverter)
	if err != nil {
		return err
	}
	if isPassThrough {
		return w.Writer.Update(ctx, obj, opts...)
	}
	if err = w.Writer.Update(ctx, converted, opts...); err != nil {
		return err
	}
	if w.convertBack {
		if err = tryConvertBack(converted, obj, rawGvk, w.SingleVersionConverter); err != nil {
			return err
		}
	}
	return nil
}

// Patch patches the given obj in the Kubernetes cluster. obj must be a
// struct pointer so that obj can be updated with the content returned by the Server.
func (w *wrapperWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	converted, rawGvk, isPassThrough, err := tryConvert(obj, w.SingleVersionConverter)
	if err != nil {
		return err
	}
	if isPassThrough {
		return w.Writer.Patch(ctx, obj, patch, opts...)
	}
	if err = w.Writer.Patch(ctx, converted, patch, opts...); err != nil {
		return err
	}
	if w.convertBack {
		if err = tryConvertBack(converted, obj, rawGvk, w.SingleVersionConverter); err != nil {
			return err
		}
	}
	return nil
}

// DeleteAllOf deletes all objects of the given type matching the given options.
func (w *wrapperWriter) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	converted, _, isPassThrough, err := tryConvert(obj, w.SingleVersionConverter)
	if err != nil {
		return err
	}
	if isPassThrough {
		return w.Writer.DeleteAllOf(ctx, obj, opts...)
	}

	return w.Writer.DeleteAllOf(ctx, converted, opts...)
}

type wrapperReader struct {
	SingleVersionConverter
	client.Reader
}

var _ ReaderWithConverter = &wrapperReader{}

func WrapReader(r client.Reader, c SingleVersionConverter) ReaderWithConverter {
	return &wrapperReader{
		SingleVersionConverter: c,
		Reader:                 r,
	}
}

// Get retrieves an obj for the given object key from the Kubernetes Cluster.
// obj must be a struct pointer so that obj can be updated with the response
// returned by the Server.
func (r *wrapperReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	converted, rawGvk, isPassThrough, err := tryConvert(obj, r.SingleVersionConverter)
	if err != nil {
		return err
	}
	if isPassThrough {
		return r.Reader.Get(ctx, key, obj)
	}
	if err = r.Reader.Get(ctx, key, converted); err != nil {
		return err
	}
	if err = tryConvertBack(converted, obj, rawGvk, r.SingleVersionConverter); err != nil {
		return err
	}
	return nil
}

// List retrieves list of objects for a given namespace and list options. On a
// successful call, Items field in the list will be populated with the
// result returned from the server.
func (r *wrapperReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	converted, rawGvk, isPassThrough, err := tryConvertList(list, r.SingleVersionConverter)
	if err != nil {
		return err
	}
	if isPassThrough {
		return r.Reader.List(ctx, list, opts...)
	}
	convertedList, ok := converted.(client.ObjectList)
	if !ok {
		return fmt.Errorf("object is not list type")
	}
	if err = r.Reader.List(ctx, convertedList, opts...); err != nil {
		return err
	}
	if err = tryConvertBack(converted, list, rawGvk, r.SingleVersionConverter); err != nil {
		return err
	}
	return nil
}

type wrapperStatusWriter struct {
	// convertBack the gate to control if there is necessary
	// to convert action result back to user
	convertBack bool

	client.StatusWriter
	SingleVersionConverter
}

var _ StatusWriterWithConverter = &wrapperStatusWriter{}

func WrapStatusWriter(r client.StatusWriter, c SingleVersionConverter, convertBack bool) StatusWriterWithConverter {
	return &wrapperStatusWriter{
		convertBack:            convertBack,
		StatusWriter:           r,
		SingleVersionConverter: c,
	}
}

func (w *wrapperStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	converted, rawGvk, isPassThrough, err := tryConvert(obj, w.SingleVersionConverter)
	if err != nil {
		return err
	}
	if isPassThrough {
		return w.StatusWriter.Update(ctx, obj, opts...)
	}
	if err = w.StatusWriter.Update(ctx, converted, opts...); err != nil {
		return err
	}
	if w.convertBack {
		if err = tryConvertBack(converted, obj, rawGvk, w.SingleVersionConverter); err != nil {
			return err
		}
	}
	return nil
}

func (w *wrapperStatusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	converted, rawGvk, isPassThrough, err := tryConvert(obj, w.SingleVersionConverter)
	if err != nil {
		return err
	}
	if isPassThrough {
		return w.StatusWriter.Patch(ctx, obj, patch, opts...)
	}
	if err = w.StatusWriter.Patch(ctx, converted, patch, opts...); err != nil {
		return err
	}
	if w.convertBack {
		if err = tryConvertBack(converted, obj, rawGvk, w.SingleVersionConverter); err != nil {
			return err
		}
	}
	return nil
}

// tryConvert do nothing if given Object can pass through
// otherwise it would try to convert it to recommend version.
func tryConvert(obj runtime.Object, c SingleVersionConverter) (client.Object, *schema.GroupVersionKind, bool, error) {
	greetBack, rawGvk, recommendGvk, err := c.ObjectGreeting(obj)
	if err != nil {
		return nil, rawGvk, false, err
	}
	if greetBack == IsPassThrough || greetBack == IsNotSupport {
		return nil, rawGvk, true, nil
	}

	converted, err := c.DirectConvert(obj, nil, recommendGvk.GroupVersion())
	if err != nil {
		return nil, rawGvk, false, err
	}

	clientObject, ok := converted.(client.Object)
	if !ok {
		return nil, rawGvk, false, fmt.Errorf("converted object (%v/%v) is not client Object type", clientObject.GetNamespace(), clientObject.GetName())
	}

	return clientObject, rawGvk, false, nil
}

// tryConvertList do nothing if given ObjectList can pass through
// otherwise it would try to convert it to recommend version.
func tryConvertList(obj runtime.Object, c SingleVersionConverter) (client.ObjectList, *schema.GroupVersionKind, bool, error) {
	greetBack, rawGvk, recommendGvk, err := c.ObjectGreeting(obj)
	if err != nil {
		return nil, rawGvk, false, err
	}
	if greetBack == IsPassThrough || greetBack == IsNotSupport {
		return nil, rawGvk, true, nil
	}

	converted, err := c.DirectConvert(obj, nil, recommendGvk.GroupVersion())
	if err != nil {
		return nil, rawGvk, false, err
	}

	clientObject, ok := converted.(client.ObjectList)
	if !ok {
		return nil, rawGvk, false, fmt.Errorf("converted object is not client Object type")
	}

	return clientObject, rawGvk, false, nil
}

// tryConvertBack would try to convert Object back to raw input by given version.
func tryConvertBack(obj runtime.Object, out runtime.Object, gvk *schema.GroupVersionKind, c SingleVersionConverter) error {
	_, err := c.DirectConvert(obj, out, gvk.GroupVersion())
	if err != nil {
		return err
	}
	return nil
}
