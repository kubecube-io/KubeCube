/*
Copyright 2021 KubeCube Authors

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

package strslice

import "k8s.io/apimachinery/pkg/util/sets"

// ContainsString functions to check and remove string from a slice of strings.
func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func RemoveString(slice []string, s string) []string {
	result := make([]string, 0)
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return result
}

// InsertString insert string without dup
func InsertString(slice []string, s string) []string {
	for _, item := range slice {
		if item == s {
			return slice
		}
	}

	slice = append(slice, s)

	return slice
}

func IsRepeatString(slice []string) bool {
	if len(slice) == 0 {
		return false
	}
	tmp := make(map[string]interface{})
	for _, value := range slice {
		if _, ok := tmp[value]; ok {
			return true
		}
		tmp[value] = sets.Empty{}
	}
	return false
}
