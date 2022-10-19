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
	"k8s.io/apimachinery/pkg/util/sets"
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

func NewEmptyFilter(installFunc ...InstallFunc) *Filter {
	scheme := runtime.NewScheme()
	install(scheme, installFunc...)
	return &Filter{Scheme: scheme}
}

func NewPageFilter(limit int, offset int, installFunc ...InstallFunc) *Filter {
	scheme := runtime.NewScheme()
	install(scheme, installFunc...)
	return &Filter{Scheme: scheme, Limit: limit, Offset: offset}
}

func NewFilter(exact map[string]sets.String,
	fuzzy map[string][]string,
	limit int,
	offset int,
	sortName string,
	sortOrder string,
	sortFunc string,
	ctx *ConverterContext,
	installFunc ...InstallFunc) *Filter {
	scheme := runtime.NewScheme()
	install(scheme, installFunc...)
	return &Filter{
		Exact:            exact,
		Fuzzy:            fuzzy,
		Limit:            limit,
		Offset:           offset,
		SortName:         sortName,
		SortOrder:        sortOrder,
		SortFunc:         sortFunc,
		Scheme:           scheme,
		ConverterContext: ctx,
	}
}

// Filter is the filter condition
type Filter struct {
	Exact     map[string]sets.String
	Fuzzy     map[string][]string
	Limit     int
	Offset    int
	SortName  string
	SortOrder string
	SortFunc  string

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
func (f *Filter) ModifyResponse(r *http.Response) error {
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

	if obj, err := f.doFilter(body); err == nil {
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

func (f *Filter) FilterObjectList(object runtime.Object) (*int, error) {
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
	first := &First{}
	var temp Handler
	temp = first
	if len(f.Exact) != 0 {
		extract := ExtractFilterChain(f.Exact)
		temp.setNext(extract)
		temp = extract
	}

	if len(f.Fuzzy) != 0 {
		fuzzy := FuzzyFilterChain(f.Fuzzy)
		temp.setNext(fuzzy)
		temp = fuzzy
	}

	if len(f.SortName) != 0 {
		sortChain := SortFilterChain(f.SortName, f.SortOrder, f.SortFunc)
		temp.setNext(sortChain)
		temp = sortChain
	}
	if f.Limit != 0 {
		pageChain := PageFilterChain(f.Limit, f.Offset)
		pageBean.Total = pageChain.total
		temp.setNext(pageChain)
		temp = pageChain
	}
	if f.ConverterContext != nil && f.EnableConvert {
		convert := ConvertFilterChain(f.EnableConvert, f.RawGvr, f.ConvertedGvr, f.Converter)
		temp.setNext(convert)
		temp = convert
	}
	temp.setNext(&Last{})
	res, err := first.handle(listObject, context.Background())
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

func (f *Filter) doFilter(data []byte) (*unstructured.Unstructured, error) {
	first := &First{}
	var temp Handler
	var total *int
	temp = first
	parseJson := ParseJsonObjChain(data, f.Scheme)
	temp.setNext(parseJson)
	temp = parseJson
	if len(f.Exact) != 0 {
		extract := ExtractFilterChain(f.Exact)
		temp.setNext(extract)
		temp = extract
	}

	if len(f.Fuzzy) != 0 {
		fuzzy := FuzzyFilterChain(f.Fuzzy)
		temp.setNext(fuzzy)
		temp = fuzzy
	}

	if len(f.SortName) != 0 {
		sortChain := SortFilterChain(f.SortName, f.SortOrder, f.SortFunc)
		temp.setNext(sortChain)
		temp = sortChain
	}
	if f.Limit != 0 {
		pageChain := PageFilterChain(f.Limit, f.Offset)
		total = pageChain.total
		temp.setNext(pageChain)
		temp = pageChain
	}
	if f.ConverterContext != nil && f.EnableConvert {
		convert := ConvertFilterChain(f.EnableConvert, f.RawGvr, f.ConvertedGvr, f.Converter)
		temp.setNext(convert)
		temp = convert
	}
	temp.setNext(&Last{})
	object, err := first.handle(nil, context.Background())
	if err != nil {
		return nil, err
	}
	if object.IsList() {
		object.Object["total"] = total
	}
	return object, nil
}
