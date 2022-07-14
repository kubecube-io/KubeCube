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

package rbac

import (
	"context"

	userinfo "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

// Interface aggregate rbac extractor and resolver
type Interface interface {
	RoleExtractor
	RoleResolver
	Authorizer
}

// IsAllowResourceAccess give a decision by auth attributes
func IsAllowResourceAccess(rbac Interface, user, resource, verb, namespace string) (bool, error) {
	a := authorizer.AttributesRecord{
		User:            &userinfo.DefaultInfo{Name: user},
		Verb:            verb,
		Namespace:       namespace,
		Resource:        resource,
		ResourceRequest: true,
	}
	d, _, err := rbac.Authorize(context.Background(), a)
	if err != nil {
		return false, err
	}

	return d == authorizer.DecisionAllow, nil
}
