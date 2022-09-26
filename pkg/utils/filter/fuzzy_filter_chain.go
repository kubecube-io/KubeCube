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

func FuzzyFilterChain(fuzzy map[string][]string) *FuzzyParam {
	return &FuzzyParam{
		fuzzy: fuzzy,
	}
}

type FuzzyParam struct {
	fuzzy   map[string][]string
	handler Handler
}

func (param *FuzzyParam) setNext(handler Handler) {
	param.handler = handler
}
func (param *FuzzyParam) handle(items []unstructured.Unstructured) ([]unstructured.Unstructured, error) {
	result := make([]unstructured.Unstructured, 0)
	// every list record
	for _, item := range items {
		flag := true
		// every fuzzy match condition
		for key, valueArray := range param.fuzzy {
			// key = metadata.xxx.xxx， multi level
			realValue, err := GetDeepValue(item, key)
			if err != nil {
				clog.Error("parse value error, %s", err.Error())
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
	return param.next(result)
}

func (param *FuzzyParam) next(items []unstructured.Unstructured) ([]unstructured.Unstructured, error) {
	if param.handler == nil {
		return items, nil
	}
	return param.handler.handle(items)
}
