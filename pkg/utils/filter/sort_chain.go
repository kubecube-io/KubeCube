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
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubecube-io/kubecube/pkg/clog"
)

type SortParam struct {
	sortName  string
	sortOrder string
	sortFunc  string
}

func SortHandler(items []unstructured.Unstructured, param *SortParam) ([]unstructured.Unstructured, error) {
	if len(items) == 0 {
		return items, nil
	}
	if len(param.sortFunc) == 0 || len(param.sortName) == 0 {
		return items, nil
	}
	sort.Slice(items, func(i, j int) bool {
		getStringFunc := func(items []unstructured.Unstructured, i int, j int) (string, string, error) {
			si, err := GetDeepValue(items[i], param.sortName)
			if err != nil {
				clog.Error("get sort value error, err: %+v", err)
				return "", "", err
			}
			if len(si) > 1 {
				clog.Error("not support array value, val: %+v", si)
				return "", "", err
			}
			sj, err := GetDeepValue(items[j], param.sortName)
			if err != nil {
				clog.Error("get sort value error, err: %+v", err)
				return "", "", err
			}
			if len(sj) > 1 {
				clog.Error("not support array value, val: %+v", sj)
				return "", "", err
			}
			before := si[0]
			after := sj[0]
			return before, after, nil
		}
		switch param.sortFunc {
		case "string":
			before, after, err := getStringFunc(items, i, j)
			if err != nil {
				return false
			}
			if param.sortFunc == "asc" {
				return strings.Compare(before, after) < 0
			} else {
				return strings.Compare(before, after) > 0
			}
		case "time":
			before, after, err := getStringFunc(items, i, j)
			if err != nil {
				return false
			}
			ti, err := time.Parse("2006-01-02T15:04:05Z", before)
			if err != nil {
				return false
			}
			tj, err := time.Parse("2006-01-02T15:04:05Z", after)
			if err != nil {
				return false
			} else if param.sortOrder == "asc" {
				return ti.Before(tj)
			} else {
				return ti.After(tj)
			}
		case "number":
			ni := GetDeepFloat64(items[i], param.sortName)
			nj := GetDeepFloat64(items[j], param.sortName)
			if param.sortOrder == "asc" {
				return ni < nj
			} else if param.sortOrder == "desc" {
				return ni > nj
			} else {
				return ni < nj
			}
		default:
			before, after, err := getStringFunc(items, i, j)
			if err != nil {
				return false
			}
			if param.sortOrder == "asc" {
				return strings.Compare(before, after) < 0
			} else {
				return strings.Compare(before, after) > 0
			}
		}
	})
	return items, nil
}
