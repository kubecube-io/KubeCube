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

package filter

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/conversion"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	Labels      = "labels"
	Annotations = "annotations"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// Filter is the filter condition
type Filter struct {
	Exact     map[string]string
	Fuzzy     map[string]string
	Limit     int
	Offset    int
	SortName  string
	SortOrder string
	SortFunc  string

	EnableFilter bool

	// ConverterContext holds methods to convert objects
	ConverterContext
}

type ConverterContext struct {
	EnableConvert bool
	RawGvr        *schema.GroupVersionResource
	ConvertedGvr  *schema.GroupVersionResource
	Converter     *conversion.VersionConverter
}

type K8sJson = map[string]interface{}
type K8sJsonArr = []interface{}

// ModifyResponse modify the response
func (f *Filter) ModifyResponse(r *http.Response) error {
	// get info from response
	var body []byte
	var err error
	codeType := r.Header.Get("Content-Encoding")
	switch codeType {
	case "gzip":
		reader, err := gzip.NewReader(r.Body)
		defer reader.Close()
		defer r.Body.Close()
		if err != nil {
			clog.Info("can not read gzip body from response, %v", err)
			return err
		}
		body, err = ioutil.ReadAll(reader)
		if err != nil {
			clog.Info("can not read gzip body from response, %v", err)
			return err
		}
	default:
		body, err = ioutil.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			clog.Info("can not read body from response, %v", err)
			return err
		}
	}

	// filter result
	result := f.FilterResult(body)
	if result == nil {
		return fmt.Errorf("filter the k8s response body fail")
	}

	// return result
	buf := bytes.NewBuffer(result)
	r.Body = ioutil.NopCloser(buf)
	r.Header["Content-Length"] = []string{fmt.Sprint(buf.Len())}
	delete(r.Header, "Content-Encoding")
	return nil
}

// FilterResult filter result by exact/fuzzy match, sort, page
func (f *Filter) FilterResult(body []byte) []byte {
	var result K8sJson
	err := json.Unmarshal(body, &result)
	if err != nil {
		clog.Info("can not parser body to map cause: %v ", err)
		// give back raw body once unmarshal failed
		return body
	}

	// k8s status response do not need filter and convert
	if items, ok := result["items"].(K8sJsonArr); ok {
		// entry here means k8s response is object list.
		// we do filter, sort and page action here.

		// match selector
		if f.EnableFilter {
			items = f.exactMatch(items)
			items = f.fuzzyMatch(items)
			result["total"] = len(items)
			// sort
			items = f.sort(items)
			// page
			items = f.page(items)
		}

		if f.EnableConvert {
			var err error
			items, err = f.ConvertItems(items...)
			if err != nil {
				clog.Info("convert items failed: %v", err)
			}
		}

		result["items"] = items
	} else if !isStatusResp(result) && f.EnableConvert {
		item, err := f.ConvertItems(result)
		if err != nil {
			clog.Info("convert object failed: %v", err)
		}
		res, err := json.Marshal(item[0])
		if err != nil {
			clog.Info("translate modify response result to json fail, %v", err)
			return body
		}
		return res
	}

	resultJson, err := json.Marshal(result)
	if err != nil {
		clog.Info("translate modify response result to json fail, %v", err)
		return body
	}
	return resultJson
}

// ConvertItems converts items by given version
func (f *Filter) ConvertItems(items ...interface{}) ([]interface{}, error) {
	res := make([]interface{}, 0, len(items))
	// todo: optimize it
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			return items, errors.New("object is not map[string]interface{}")
		}
		u := &unstructured.Unstructured{Object: m}
		if u.GetAPIVersion() == "" {
			u.SetAPIVersion(f.ConvertedGvr.GroupVersion().String())
		}
		if u.GetKind() == "" {
			gvk, err := conversion.Gvr2Gvk(f.Converter.RestMapper, f.ConvertedGvr)
			if err != nil {
				return nil, err
			}
			u.SetKind(gvk.Kind)
		}
		out, err := f.Converter.DirectConvert(u, nil, f.RawGvr.GroupVersion())
		if err != nil {
			return items, err
		}
		res = append(res, out)
	}
	return res, nil
}

// isStatusResp tells if k8s response is just only status
func isStatusResp(r K8sJson) bool {
	ok1 := false
	ok2 := false
	if kind, ok := r["kind"].(string); ok {
		if kind == "Status" {
			ok1 = true
		}
	}
	if apiVersion, ok := r["apiVersion"].(v1.StatusReason); ok {
		if apiVersion == "v1" {
			ok2 = true
		}
	}
	return ok1 && ok2
}

// FilterResultToMap filter result by exact/fuzzy match, sort, page
func (f *Filter) FilterResultToMap(body []byte) K8sJson {
	var result K8sJson
	err := json.Unmarshal(body, &result)
	if err != nil {
		clog.Error("can not parser body to map, %v ", err)
		return nil
	}

	// list type
	if result["items"] != nil {
		if items, ok := result["items"].(K8sJsonArr); ok {
			// match selector
			items = f.exactMatch(items)
			items = f.fuzzyMatch(items)
			result["total"] = len(items)
			// sort
			items = f.sort(items)
			// page
			items = f.page(items)
			result["items"] = items
		}
	}

	return result
}

// exact search
func (f *Filter) exactMatch(items K8sJsonArr) (result K8sJsonArr) {
	if len(f.Exact) == 0 {
		return items
	}

	// every list record
	for _, item := range items {
		flag := true
		// every exact match condition
		for key, value := range f.Exact {
			// key = .metadata.xxx.xxx， multi level
			realValue, err := GetDeepValue(item, key)
			if err != nil {
				clog.Error("parse value error, %+v", err)
				flag = false
				break
			}
			// if one condition not match
			valCheck := false
			for _, v := range realValue {
				if strings.EqualFold(v, value) {
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
	return
}

// fuzzy search
func (f *Filter) fuzzyMatch(items K8sJsonArr) (result K8sJsonArr) {
	if len(f.Fuzzy) == 0 {
		return items
	}

	// every list record
	for _, item := range items {
		flag := true
		// every fuzzy match condition
		for key, value := range f.Fuzzy {
			// key = metadata.xxx.xxx， multi level
			realValue, err := GetDeepValue(item, key)
			if err != nil {
				clog.Error("parse value error, %+v", err)
				flag = false
				break
			}
			// if one condition not match
			valCheck := false
			for _, v := range realValue {
				if strings.Contains(v, value) {
					valCheck = true
					break
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
	return
}

// sort by .metadata.name/.metadata.creationTimestamp
func (f *Filter) sort(items K8sJsonArr) K8sJsonArr {
	if len(items) == 0 {
		return items
	}

	sort.Slice(items, func(i, j int) bool {
		si, err := GetDeepValue(items[i], f.SortName)
		if err != nil {
			clog.Error("get sort value error, err: %+v", err)
			return false
		}
		if len(si) > 1 {
			clog.Error("not support array value, val: %+v", si)
			return false
		}
		sj, err := GetDeepValue(items[j], f.SortName)
		if err != nil {
			clog.Error("get sort value error, err: %+v", err)
			return false
		}
		if len(sj) > 1 {
			clog.Error("not support array value, val: %+v", sj)
			return false
		}
		before := si[0]
		after := sj[0]
		switch f.SortFunc {
		case "string":
			if f.SortOrder == "asc" {
				return strings.Compare(before, after) < 0
			} else {
				return strings.Compare(before, after) > 0
			}
		case "time":
			ti, err := time.Parse("2006-01-02T15:04:05Z", before)
			if err != nil {
				return false
			}
			tj, err := time.Parse("2006-01-02T15:04:05Z", after)
			if err != nil {
				return false
			} else if f.SortOrder == "asc" {
				return ti.Before(tj)
			} else {
				return ti.After(tj)
			}
		case "number":
			ni := GetDeepFloat64(items[i], f.SortName)
			nj := GetDeepFloat64(items[j], f.SortName)
			if f.SortOrder == "asc" {
				return ni < nj
			} else if f.SortOrder == "desc" {
				return ni > nj
			} else {
				return ni < nj
			}
		default:
			if f.SortOrder == "asc" {
				return strings.Compare(before, after) < 0
			} else {
				return strings.Compare(before, after) > 0
			}
		}

	})
	return items
}

// page
func (f *Filter) page(items K8sJsonArr) K8sJsonArr {
	if len(items) == 0 {
		return items
	}

	size := len(items)
	if f.Offset >= size {
		return items[0:0]
	}
	end := f.Offset + f.Limit
	if end > size {
		end = size
	}
	return items[f.Offset:end]
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
	case K8sJson:
		info := item.(K8sJson)
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
	// if the value is []map[string]interface{}, so we need get all value which key is, so just foreach it
	case K8sJsonArr:
		arr := item.(K8sJsonArr)
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
	// for other value,we not support now
	default:
		return nil, fmt.Errorf("not map value of field is not support")
	}

}

// GetDeepFloat64 get float64 value by metadata.xx.xx.xx, multi level key
func GetDeepFloat64(item interface{}, keyStr string) (value float64) {
	defer func() {
		if err := recover(); err != nil {
			value = 0
			return
		}
	}()

	temp := item.(K8sJson)
	// key = metadata.xxx.xxx， multi level
	keys := strings.Split(keyStr, ".")
	n := len(keys)
	i := 0
	for ; n > 0 && i < n-1; i++ {
		temp = temp[keys[i]].(K8sJson)
		if keys[i] == Labels || keys[i] == Annotations {
			i++
			break
		}
	}
	key := strings.Join(keys[i:], ".")
	value = temp[key].(float64)
	return
}

// GetDeepMap get map by spec.selector.matchLabels={xx= xx}
func GetDeepMap(item interface{}, keyStr string) (value K8sJson) {
	defer func() {
		if err := recover(); err != nil {
			value = nil
			return
		}
	}()

	temp := item.(K8sJson)
	// key = spec.selector.matchLabels， multi level
	keys := strings.Split(keyStr, ".")
	n := len(keys)
	i := 0
	for ; n > 0 && i < n-1; i++ {
		temp = temp[keys[i]].(K8sJson)
		if keys[i] == Labels || keys[i] == Annotations {
			i++
			break
		}
	}
	key := strings.Join(keys[i:], ".")
	value = temp[key].(K8sJson)
	return
}

// GetDeepArray get metadata.ownerReference[0]
func GetDeepArray(item interface{}, keyStr string) (value K8sJsonArr) {
	defer func() {
		if err := recover(); err != nil {
			value = nil
			return
		}
	}()

	temp := item.(K8sJson)
	// key = metadata.ownerReference[0]， multi level
	keys := strings.Split(keyStr, ".")
	n := len(keys)
	i := 0
	for ; n > 0 && i < n-1; i++ {
		temp = temp[keys[i]].(K8sJson)
		if keys[i] == Labels || keys[i] == Annotations {
			i++
			break
		}
	}
	key := strings.Join(keys[i:], ".")
	value = temp[key].(K8sJsonArr)
	return
}
