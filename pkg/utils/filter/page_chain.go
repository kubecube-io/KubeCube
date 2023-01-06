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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func PageHandler(items []unstructured.Unstructured, limit int, offset int) ([]unstructured.Unstructured, error) {

	if len(items) == 0 {
		return items, nil
	}
	if limit == 0 {
		return items, nil
	}
	size := len(items)
	if offset >= size {
		return items, nil
	}
	end := offset + limit
	if end > size {
		end = size
	}
	return items[offset:end], nil
}
