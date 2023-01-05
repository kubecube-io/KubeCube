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
	"github.com/kubecube-io/kubecube/pkg/conversion"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ConvertParam struct {
	enableConvert bool
	rawGvr        *schema.GroupVersionResource
	convertedGvr  *schema.GroupVersionResource
	converter     *conversion.VersionConverter
}

func ConvertHandler(items []unstructured.Unstructured, param *ConvertParam) ([]unstructured.Unstructured, error) {
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
	return res, nil
}
