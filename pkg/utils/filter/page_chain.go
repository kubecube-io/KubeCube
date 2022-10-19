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
)

func PageFilterChain(limit int, offset int) *PageParam {
	return &PageParam{
		limit:  limit,
		offset: offset,
		total:  new(int),
	}
}

type PageParam struct {
	limit   int
	offset  int
	total   *int
	handler Handler
}

func (param *PageParam) setNext(handler Handler) {
	param.handler = handler
}
func (param *PageParam) handle(items []unstructured.Unstructured, ctx context.Context) (*unstructured.Unstructured, error) {
	if !ctx.Value(isObjectIsList).(bool) {
		return param.next(items, ctx)
	}
	*param.total = len(items)
	if len(items) == 0 {
		return param.next(items, ctx)
	}
	size := len(items)
	if param.offset >= size {
		return param.next(items[0:0], ctx)
	}
	end := param.offset + param.limit
	if end > size {
		end = size
	}
	return param.next(items[param.offset:end], ctx)
}

func (param *PageParam) next(items []unstructured.Unstructured, ctx context.Context) (*unstructured.Unstructured, error) {
	if param.handler == nil {
		return GetUnstructured(items), nil
	}
	return param.handler.handle(items, ctx)
}
