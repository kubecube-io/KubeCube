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
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/kubecube-io/kubecube/pkg/clog"
)

func ExactFilter(items []unstructured.Unstructured, exact map[string]sets.String) ([]unstructured.Unstructured, error) {
	if len(items) < 1 {
		return items, nil
	}
	if len(exact) < 1 {
		return items, nil
	}
	result := make([]unstructured.Unstructured, 0)
	// every list record
	for _, item := range items {
		flag := true
		// every exact match condition
		for key, value := range exact {
			// key = .metadata.xxx.xxx， multi level
			realValue, err := GetDeepValue(item, key)
			if err != nil {
				clog.Debug("parse value error, %s", err)
				flag = false
				break
			}
			// if one condition not match
			valCheck := false
			for _, v := range realValue {
				if value.Has(v) {
					valCheck = true
					break
				}
			}
			if valCheck != true {
				flag = false
				break
			}
		}
		// if every exact condition match
		if flag {
			result = append(result, item)
		}
	}
	return result, nil
}
