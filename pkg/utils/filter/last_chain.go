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

type Last struct {
	handler Handler
}

func (last *Last) setNext(handler Handler) {
	last.handler = handler
}
func (last *Last) handle(items []unstructured.Unstructured, ctx context.Context) (*unstructured.Unstructured, error) {
	if ctx.Value(isObjectIsList).(bool) {
		return GetUnstructured(items), nil
	}
	if len(items) == 0 {
		return nil, nil
	}
	return &items[0], nil
}

func (last *Last) next(_ []unstructured.Unstructured, ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}
