/*
Copyright 2023 KubeCube Authors

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
	"errors"
	"reflect"
	"strings"
)

type FuzzyMatchCondition struct {
	SearchStr      string
	FieldExtractor func(obj interface{}) string
}

func FuzzyMatch(slice interface{}, conds ...FuzzyMatchCondition) ([]interface{}, error) {
	var results []interface{}

	value := reflect.ValueOf(slice)

	if value.Kind() != reflect.Slice {
		return results, errors.New("search: not a slice")
	}

	for i := 0; i < value.Len(); i++ {
		element := value.Index(i).Interface()
		matched := true
		for _, cond := range conds {
			value := cond.FieldExtractor(element)
			if strings.Contains(value, cond.SearchStr) {
				matched = true
			} else {
				matched = false
			}
		}
		if matched {
			results = append(results, element)
		}
	}

	return results, nil
}
