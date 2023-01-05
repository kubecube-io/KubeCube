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
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubecube-io/kubecube/pkg/clog"
)

type PageBean struct {
	Total int         `json:"total"`
	Items interface{} `json:"items"`
}

// GetDeepValue get value by metadata.xx.xx.xx, multi level key
func GetDeepValue(item interface{}, keyStr string) ([]string, error) {
	fields := strings.Split(keyStr, ".")
	n := len(fields)

	if n < 1 {
		return nil, fmt.Errorf("keyStr format invilid")
	}

	v, err := getRes(item, 0, fields)
	if err != nil {
		return nil, err
	}

	switch v.(type) {
	case string:
		return []string{
			v.(string),
		}, nil
	case []interface{}:
		array := v.([]interface{})
		var result []string
		for _, value := range array {
			if val, ok := value.(string); ok {
				result = append(result, val)
			}
		}
		return result, nil
	case []string:
		return v.([]string), nil
	default:
		return nil, fmt.Errorf("only support string or string array value, the value is: %+v", v)
	}
}

func getRes(item interface{}, index int, fields []string) (interface{}, error) {
	switch item.(type) {
	// if this value is map[string]interface{}, we need to get the value which key is, and return next index to get next key
	case map[string]interface{}:
		info := item.(map[string]interface{})
		n := len(fields)
		//In special cases, such as label, key exists "." At this time, the following key is directly spliced into a complete key and the value is obtained
		if fields[index] == Labels || fields[index] == Annotations {
			key := strings.Join(fields[index+1:], ".")
			// any field not found return directly
			next, ok := info[fields[index]]
			if !ok {
				return nil, fmt.Errorf("field %v not exsit", fields[index])
			}
			return getRes(next, 0, []string{key})
		}
		// the end out of loop
		if index == n-1 {
			v, ok := info[fields[index]]
			if !ok {
				return nil, fmt.Errorf("field %v not exsit", fields[index])
			}
			return v, nil
		}

		// any field not found return directly
		next, ok := info[fields[index]]
		if !ok {
			return nil, fmt.Errorf("field %v not exsit", fields[index])
		}
		return getRes(next, index+1, fields)
	case unstructured.Unstructured:
		info := item.(unstructured.Unstructured)
		n := len(fields)
		//In special cases, such as label, key exists "." At this time, the following key is directly spliced into a complete key and the value is obtained
		if fields[index] == Labels || fields[index] == Annotations {
			key := strings.Join(fields[index+1:], ".")
			// any field not found return directly
			next, ok := info.Object[fields[index]]
			if !ok {
				return nil, fmt.Errorf("field %v not exsit", fields[index])
			}
			return getRes(next, 0, []string{key})
		}
		// the end out of loop
		if index == n-1 {
			v, ok := info.Object[fields[index]]
			if !ok {
				return nil, fmt.Errorf("field %v not exsit", fields[index])
			}
			return v, nil
		}

		// any field not found return directly
		next, ok := info.Object[fields[index]]
		if !ok {
			return nil, fmt.Errorf("field %v not exsit", fields[index])
		}
		return getRes(next, index+1, fields)
	// if the value is []map[string]interface{}, so we need get all value which key is, so just foreach it
	case []interface{}:
		arr := item.([]interface{})
		var result []interface{}
		for _, info := range arr {
			res, err := getRes(info, index, fields)
			if err != nil {
				clog.Error(err.Error())
				continue
			}
			result = append(result, res)
		}
		return result, nil
	case unstructured.UnstructuredList:
		arr := item.(unstructured.UnstructuredList)
		var result []interface{}
		for _, info := range arr.Items {
			res, err := getRes(info, index, fields)
			if err != nil {
				clog.Error(err.Error())
				continue
			}
			result = append(result, res)
		}
		return result, nil
	// for other value,we not support now
	default:
		return nil, fmt.Errorf("not map value of field is not support")
	}

}

// GetDeepFloat64 get float64 value by metadata.xx.xx.xx, multi level key
func GetDeepFloat64(item unstructured.Unstructured, keyStr string) (value float64) {
	defer func() {
		if err := recover(); err != nil {
			value = 0
			return
		}
	}()

	keys := strings.Split(keyStr, ".")
	n := len(keys)
	i := 0
	temp := item.Object
	for ; n > 0 && i < n-1; i++ {
		temp = temp[keys[i]].(map[string]interface{})
		if keys[i] == Labels || keys[i] == Annotations {
			i++
			break
		}
	}
	key := strings.Join(keys[i:], ".")
	value = temp[key].(float64)
	return
}
