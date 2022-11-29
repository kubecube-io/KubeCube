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
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubecube-io/kubecube/pkg/clog"
)

func FuzzyFilter(items []unstructured.Unstructured, fuzzy map[string][]string) ([]unstructured.Unstructured, error) {
	if len(items) < 1 {
		return items, nil
	}
	if len(fuzzy) < 1 {
		return items, nil
	}
	result := make([]unstructured.Unstructured, 0)
	// every list record
	for _, item := range items {
		flag := true
		// every fuzzy match condition
		for key, valueArray := range fuzzy {
			// key = metadata.xxx.xxx， multi level
			realValue, err := GetDeepValue(item, key)
			if err != nil {
				clog.Debug("parse value error, %s", err)
				flag = false
				break
			}
			// if one condition not match
			valCheck := false
			for _, v := range realValue {
				for _, value := range valueArray {
					if strings.Contains(v, value) {
						valCheck = true
						break
					}
				}
			}
			if valCheck != true {
				flag = false
				break
			}
		}
		// if every fuzzy condition match
		if flag {
			result = append(result, item)
		}
	}
	return result, nil
}
