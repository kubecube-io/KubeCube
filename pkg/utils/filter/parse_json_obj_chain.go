package filter

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"github.com/kubecube-io/kubecube/pkg/clog"
)

func ParseJsonObjChain(data []byte, scheme *runtime.Scheme) *ParseJsonObjParam {
	return &ParseJsonObjParam{
		data:   data,
		scheme: scheme,
	}
}

type ParseJsonObjParam struct {
	data    []byte
	scheme  *runtime.Scheme
	handler Handler
}

func (param *ParseJsonObjParam) setNext(handler Handler) {
	param.handler = handler
}
func (param *ParseJsonObjParam) handle(_ []unstructured.Unstructured) ([]unstructured.Unstructured, error) {
	codecFactory := serializer.NewCodecFactory(param.scheme)
	decoder := codecFactory.UniversalDecoder()
	internalObject, gvr, err := decoder.Decode(param.data, nil, nil)
	if err != nil {
		clog.Error("can not parser data to internalObject cause: %v ", err)
		return nil, err
	}
	object := unstructured.Unstructured{}
	err = param.scheme.Convert(internalObject, &object, gvr.GroupVersion())
	if err != nil {
		return nil, err
	}
	var listObject []unstructured.Unstructured
	if object.IsList() {
		list, err := object.ToList()
		if err != nil {
			return nil, err
		}
		listObject = list.Items
	} else {
		listObject = []unstructured.Unstructured{object}
	}
	return param.next(listObject)
}

func (param *ParseJsonObjParam) next(items []unstructured.Unstructured) ([]unstructured.Unstructured, error) {
	if param.handler == nil {
		return items, nil
	}
	return param.handler.handle(items)
}
