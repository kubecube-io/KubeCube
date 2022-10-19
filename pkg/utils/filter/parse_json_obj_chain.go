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

package filter

import (
	"context"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"github.com/kubecube-io/kubecube/pkg/clog"
)

func ParseJsonObjChain(data []byte, scheme *runtime.Scheme) *ParseJsonObjParam {
	return &ParseJsonObjParam{
		data:   data,
		scheme: scheme,
	}
}

type ParseJsonObjParam struct {
	data    []byte
	scheme  *runtime.Scheme
	handler Handler
}

func (param *ParseJsonObjParam) setNext(handler Handler) {
	param.handler = handler
}
func (param *ParseJsonObjParam) handle(_ []unstructured.Unstructured, ctx context.Context) (*unstructured.Unstructured, error) {
	codecFactory := serializer.NewCodecFactory(param.scheme)
	decoder := codecFactory.UniversalDecoder()
	object := unstructured.Unstructured{}
	_, _, err := decoder.Decode(param.data, nil, &object)
	if err != nil {
		clog.Error("can not parser data to internalObject cause: %v ", err)
		return nil, err
	}
	var listObject []unstructured.Unstructured
	ctx = context.WithValue(ctx, isObjectIsList, object.IsList())
	if object.IsList() {
		list, err := object.ToList()
		if err != nil {
			return nil, err
		}
		listObject = list.Items
	} else {
		listObject = []unstructured.Unstructured{object}
	}
	return param.next(listObject, ctx)
}

func (param *ParseJsonObjParam) next(items []unstructured.Unstructured, ctx context.Context) (*unstructured.Unstructured, error) {
	if param.handler == nil {
		return GetUnstructured(items), nil
	}
	return param.handler.handle(items, ctx)
}
