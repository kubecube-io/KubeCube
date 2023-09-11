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

package selector

import (
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
)

const k8sLabelPrefix = "metadata.labels"

// ParseSelector exact query：selector=key1=value1,key2=value2,key3=value3
// fuzzy query：selector=key1~value1,key2~value2,key3~value3
// multi query: selector=key=value1|value2|value3
// support mixed query：selector=key1~value1,key2=value2,key3=value3
func ParseSelector(selectorStr string) (exact map[string]sets.Set[string], fuzzy map[string][]string) {
	exact = make(map[string]sets.Set[string])
	fuzzy = make(map[string][]string)

	if selectorStr == "" {
		return
	}

	labels := strings.Split(selectorStr, ",")
	for _, label := range labels {
		if i := strings.IndexAny(label, "~="); i > 0 {
			if label[i] == '=' {
				values := strings.Split(label[i+1:], "|")
				set := sets.Set[string]{}
				for _, value := range values {
					set.Insert(value)
				}
				exact[label[:i]] = set
			} else {
				values := strings.Split(label[i+1:], "|")
				fuzzy[label[:i]] = values
			}
		}
	}

	return
}

// ParseLabelSelector exact query：selector=key1=value1,key2=value2,key3=value3
// fuzzy query：selector=key1~value1,key2~value2,key3~value3
// multi query: selector=key=value1|value2|value3
// support mixed query：selector=key1~value1,key2=value2,key3=value3
func ParseLabelSelector(selectorStr string) (labelSelector map[string][]string) {
	labelSelector = make(map[string][]string, 0)

	if selectorStr == "" {
		return
	}

	labels := strings.Split(selectorStr, ",")
	for _, label := range labels {
		if i := strings.IndexAny(label, "~="); i > 0 {
			if label[i] == '=' {
				values := strings.Split(label[i+1:], "|")
				var set []string
				for _, value := range values {
					set = append(set, value)
				}
				// can be convert labelSelector
				if strings.HasPrefix(label[:i], k8sLabelPrefix) {
					temp := label[:i]
					labelValue := strings.Split(temp, k8sLabelPrefix+".")
					labelSelector[labelValue[1]] = set
				}
			}
		}
	}

	return
}
