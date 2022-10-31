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
	"context"
	"fmt"
	"io/ioutil"

	jsoniter "github.com/json-iterator/go"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"net/http"

	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/conversion"
)

const (
	Labels      = "labels"
	Annotations = "annotations"
)

var (
	json = jsoniter.ConfigCompatibleWithStandardLibrary
)

func NewFilter(
	ctx *ConverterContext,
	installFunc ...InstallFunc) *Filter {
	scheme := runtime.NewScheme()
	install(scheme, installFunc...)
	return &Filter{
		Scheme:           scheme,
		ConverterContext: ctx,
	}
}

// Filter is the filter condition
type Filter struct {
	Scheme *runtime.Scheme
	// ConverterContext holds methods to convert objects
	*ConverterContext
}

type ConverterContext struct {
	EnableConvert bool
	RawGvr        *schema.GroupVersionResource
	ConvertedGvr  *schema.GroupVersionResource
	Converter     *conversion.VersionConverter
}

// ModifyResponse modify the response
func (f *Filter) ModifyResponse(r *http.Response, filterCondition *Condition) error {
	// get info from response
	var body []byte
	var err error
	codeType := r.Header.Get("Content-Encoding")
	switch codeType {
	case "gzip":
		reader, err := gzip.NewReader(r.Body)
		defer reader.Close()
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
		if err != nil {
			clog.Info("can not read body from response, %v", err)
			return err
		}
	}

	// todo create index
	if obj, err := f.doFilter(body, filterCondition); err == nil {
		// return result
		marshal, err := json.Marshal(obj)
		if err != nil {
			clog.Error("modify response failed: %s", err.Error())
		}
		body = marshal
	} else {
		clog.Warn("modify response failed: %s", err.Error())
	}
	buf := bytes.NewBuffer(body)
	r.Body = ioutil.NopCloser(buf)
	r.Header["Content-Length"] = []string{fmt.Sprint(buf.Len())}
	delete(r.Header, "Content-Encoding")
	return nil
}

func (f *Filter) FilterObjectList(object runtime.Object, filterCondition *Condition) (*int, error) {
	pageBean := PageBean{}
	version := object.GetObjectKind().GroupVersionKind().Version
	u := unstructured.Unstructured{}
	err := f.Scheme.Convert(object, &u, version)
	if err != nil {
		return nil, err
	}
	if !u.IsList() {
		return nil, nil
	}
	var listObject []unstructured.Unstructured
	list, err := u.ToList()
	if err != nil {
		return nil, err
	}
	listObject = list.Items
	if len(listObject) == 0 {
		return nil, nil
	}
	version = listObject[0].GroupVersionKind().Version
	var temp Handler
	if len(filterCondition.Exact) != 0 {
		extract := ExtractFilterChain(filterCondition.Exact)
		temp.setNext(extract)
		temp = extract
	}

	if len(filterCondition.Fuzzy) != 0 {
		fuzzy := FuzzyFilterChain(filterCondition.Fuzzy)
		temp.setNext(fuzzy)
		temp = fuzzy
	}

	if len(filterCondition.SortName) != 0 {
		sortChain := SortFilterChain(filterCondition.SortName, filterCondition.SortOrder, filterCondition.SortFunc)
		temp.setNext(sortChain)
		temp = sortChain
	}
	if filterCondition.Limit != 0 {
		pageChain := PageFilterChain(filterCondition.Limit, filterCondition.Offset)
		pageBean.Total = pageChain.total
		temp.setNext(pageChain)
		temp = pageChain
	}
	if f.ConverterContext != nil && f.EnableConvert {
		convert := ConvertFilterChain(f.EnableConvert, f.RawGvr, f.ConvertedGvr, f.Converter)
		temp.setNext(convert)
		temp = convert
	}
	res, err := temp.handle(listObject, context.WithValue(context.Background(), isObjectIsList, true))
	if err != nil {
		return nil, err
	}
	obj, err := res.ToList()
	if err != nil {
		return nil, err
	}
	list.Items = obj.Items
	err = f.Scheme.Convert(list, object, version)
	if err != nil {
		return nil, err
	}
	return pageBean.Total, nil
}

func (f *Filter) doFilter(data []byte, filterCondition *Condition) (*unstructured.Unstructured, error) {
	var temp Handler
	var total *int
	parseJson := ParseJsonObjChain(data, f.Scheme)
	temp.setNext(parseJson)
	temp = parseJson
	if len(filterCondition.Exact) != 0 {
		extract := ExtractFilterChain(filterCondition.Exact)
		temp.setNext(extract)
		temp = extract
	}

	if len(filterCondition.Fuzzy) != 0 {
		fuzzy := FuzzyFilterChain(filterCondition.Fuzzy)
		temp.setNext(fuzzy)
		temp = fuzzy
	}

	if len(filterCondition.SortName) != 0 {
		sortChain := SortFilterChain(filterCondition.SortName, filterCondition.SortOrder, filterCondition.SortFunc)
		temp.setNext(sortChain)
		temp = sortChain
	}
	if filterCondition.Limit != 0 {
		pageChain := PageFilterChain(filterCondition.Limit, filterCondition.Offset)
		total = pageChain.total
		temp.setNext(pageChain)
		temp = pageChain
	}
	if f.ConverterContext != nil && f.EnableConvert {
		convert := ConvertFilterChain(f.EnableConvert, f.RawGvr, f.ConvertedGvr, f.Converter)
		temp.setNext(convert)
		temp = convert
	}
	object, err := temp.handle(nil, context.Background())
	if err != nil {
		return nil, err
	}
	if object.IsList() {
		object.Object["total"] = total
	}
	return object, nil
}
