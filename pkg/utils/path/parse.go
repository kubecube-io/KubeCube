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

package path

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ResourceIdentity struct {
	Name         string
	Namespace    string
	IsCoreApi    bool
	IsNamespaced bool
	Gvr          schema.GroupVersionResource
}

// Parse parse k8s api url into gvr
func Parse(url string) (*ResourceIdentity, error) {
	invalidUrlErr := fmt.Errorf("can not parse url: %s", url)

	const (
		coreApiPrefix    = "/api/"
		nonCoreApiPrefix = "/apis/"
		nsSubString      = "/namespaces/"
	)

	isCoreApi, isNonCoreApi := strings.HasPrefix(url, coreApiPrefix), strings.HasPrefix(url, nonCoreApiPrefix)

	ss := strings.Split(strings.TrimPrefix(url, "/"), "/")
	var isNamespaced bool
	if len(ss) > 4 && strings.Contains(url, nsSubString) {
		isNamespaced = true
	}

	ri := &ResourceIdentity{
		Gvr:          schema.GroupVersionResource{},
		IsCoreApi:    isCoreApi,
		IsNamespaced: isNamespaced,
	}
	switch {
	case isCoreApi && isNamespaced:
		// like: /api/v1/namespaces/{namespace}/pods
		if len(ss) < 5 {
			return nil, invalidUrlErr
		}
		ri.Gvr.Resource = ss[4]
		ri.Gvr.Version = ss[1]
		ri.Namespace = ss[3]
		if len(ss) > 5 {
			ri.Name = ss[5]
		}
	case isCoreApi && !isNamespaced:
		// like: /api/v1/namespaces/{name}
		if len(ss) < 3 {
			return nil, invalidUrlErr
		}
		ri.Gvr.Version = ss[1]
		ri.Gvr.Resource = ss[2]
		if len(ss) > 3 {
			ri.Name = ss[3]
		}
	case isNonCoreApi && isNamespaced:
		// like: /apis/batch/v1/namespaces/{namespace}/jobs
		if len(ss) < 6 {
			return nil, invalidUrlErr
		}
		ri.Gvr.Group = ss[1]
		ri.Gvr.Version = ss[2]
		ri.Gvr.Resource = ss[5]
		if len(ss) > 6 {
			ri.Name = ss[6]
		}
	case isNonCoreApi && !isNamespaced:
		// like: /apis/rbac.authorization.k8s.io/v1/clusterroles/{name}
		if len(ss) < 4 {
			return nil, invalidUrlErr
		}
		ri.Gvr.Group = ss[1]
		ri.Gvr.Version = ss[2]
		ri.Gvr.Resource = ss[3]
		if len(ss) > 4 {
			ri.Name = ss[4]
		}
	default:
		return nil, invalidUrlErr
	}

	return ri, nil
}
