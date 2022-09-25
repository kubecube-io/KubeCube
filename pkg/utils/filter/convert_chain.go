package filter

import (
	"github.com/kubecube-io/kubecube/pkg/conversion"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func ConvertFilterChain(enableConvert bool, rawGvr *schema.GroupVersionResource, convertedGvr *schema.GroupVersionResource, converter *conversion.VersionConverter) *ConvertParam {
	return &ConvertParam{
		enableConvert: enableConvert,
		rawGvr:        rawGvr,
		convertedGvr:  convertedGvr,
		converter:     converter,
	}
}

type ConvertParam struct {
	enableConvert bool
	rawGvr        *schema.GroupVersionResource
	convertedGvr  *schema.GroupVersionResource
	converter     *conversion.VersionConverter
	handler       Handler
}

func (param *ConvertParam) setNext(handler Handler) {
	param.handler = handler
}
func (param *ConvertParam) handle(items []unstructured.Unstructured) ([]unstructured.Unstructured, error) {
	res := make([]unstructured.Unstructured, 0, len(items))
	for _, u := range items {
		if u.GetAPIVersion() == "" {
			u.SetAPIVersion(param.convertedGvr.GroupVersion().String())
		}
		if u.GetKind() == "" {
			gvk, err := conversion.Gvr2Gvk(param.converter.RestMapper, param.convertedGvr)
			if err != nil {
				return nil, err
			}
			u.SetKind(gvk.Kind)
		}
		out := unstructured.Unstructured{}
		_, err := param.converter.DirectConvert(&u, &out, param.rawGvr.GroupVersion())
		if err != nil {
			return items, err
		}
		res = append(res, out)
	}
	return param.next(res)
}

func (param *ConvertParam) next(items []unstructured.Unstructured) ([]unstructured.Unstructured, error) {
	if param.handler == nil {
		return items, nil
	}
	return param.handler.handle(items)
}
