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

func (f *Filter) doFilter(data []byte, filterCondition *Condition) (*unstructured.Unstructured, error) {
	obj, err := ParseJsonDataHandler(data, f.Scheme)
	if err != nil {
		return nil, err
	}
	res := unstructured.Unstructured{}
	if !obj.IsList() {
		err := f.convertRes(obj, res, obj.GroupVersionKind().Version)
		if err != nil {
			return nil, err
		}
		return &res, nil
	}
	object, err := obj.ToList()
	if err != nil {
		return nil, err
	}
	if len(object.Items) == 0 {
		return obj, nil
	}
	version := object.Items[0].GroupVersionKind().Version
	temp, total, err := f.filter(object.Items, filterCondition)
	if err != nil {
		return nil, err
	}
	object.Items = temp
	object.Object["total"] = total
	err = f.convertRes(object, &res, version)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (f *Filter) FilterObjectList(object runtime.Object, filterCondition *Condition) (int, error) {
	version := object.GetObjectKind().GroupVersionKind().Version
	unstructuredObj := unstructured.Unstructured{}
	err := f.Scheme.Convert(object, &unstructuredObj, version)
	if err != nil {
		return 0, err
	}
	if !unstructuredObj.IsList() {
		return 0, nil
	}
	list, err := unstructuredObj.ToList()
	if err != nil {
		return 0, err
	}
	if len(list.Items) == 0 {
		return 0, nil
	}
	version = list.Items[0].GroupVersionKind().Version
	temp, total, err := f.filter(list.Items, filterCondition)
	if err != nil {
		return 0, err
	}
	list.Items = temp
	err = f.convertRes(list, object, version)
	return total, err
}

func (f *Filter) filter(listObject []unstructured.Unstructured, filterCondition *Condition) ([]unstructured.Unstructured, int, error) {
	listObject, err := ExactFilter(listObject, filterCondition.Exact)
	if err != nil {
		clog.Debug("exact filter error: %s", err)
	}

	listObject, err = FuzzyFilter(listObject, filterCondition.Fuzzy)
	if err != nil {
		clog.Debug("fuzzy filter error: %s", err)
	}

	sortParam := SortParam{
		sortName:  filterCondition.SortName,
		sortFunc:  filterCondition.SortFunc,
		sortOrder: filterCondition.SortOrder,
	}
	listObject, err = SortHandler(listObject, &sortParam)
	if err != nil {
		clog.Debug("sort items error: %s", err)
	}

	total := len(listObject)
	listObject, err = PageHandler(listObject, filterCondition.Limit, filterCondition.Offset)
	if err != nil {
		clog.Debug("page items error: %s", err)
	}

	if f.ConverterContext != nil && f.EnableConvert {
		convertParam := ConvertParam{
			enableConvert: f.EnableConvert,
			rawGvr:        f.RawGvr,
			convertedGvr:  f.ConvertedGvr,
			converter:     f.Converter,
		}
		listObject, err = ConvertHandler(listObject, &convertParam)
		if err != nil {
			clog.Error("convert obj error: %s", err)
		}
	}
	return listObject, total, nil
}

func (f *Filter) convertRes(in, out, version interface{}) error {
	err := f.Scheme.Convert(in, out, version)
	if err != nil {
		return err
	}
	return nil
}
