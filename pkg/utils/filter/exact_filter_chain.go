package filter

import (
	"github.com/kubecube-io/kubecube/pkg/clog"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
)

func ExtractFilterChain(exact map[string]sets.String) *ExtractParam {
	return &ExtractParam{
		exact: exact,
	}
}

type ExtractParam struct {
	exact   map[string]sets.String
	handler Handler
}

func (param *ExtractParam) setNext(handler Handler) {
	param.handler = handler
}
func (param *ExtractParam) handle(items []unstructured.Unstructured) ([]unstructured.Unstructured, error) {
	result := make([]unstructured.Unstructured, 0)
	// every list record
	for _, item := range items {
		flag := true
		// every exact match condition
		for key, value := range param.exact {
			// key = .metadata.xxx.xxx， multi level
			realValue, err := GetDeepValue(item, key)
			if err != nil {
				clog.Warn("parse value error, %s", err.Error())
				flag = false
				break
			}
			// if one condition not match
			valCheck := false
			for _, v := range realValue {
				if value.Has(v) {
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
	return param.next(result)
}

func (param *ExtractParam) next(items []unstructured.Unstructured) ([]unstructured.Unstructured, error) {
	if param.handler == nil {
		return items, nil
	}
	return param.handler.handle(items)
}
