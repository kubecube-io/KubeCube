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

package resources

import (
	"context"

	"github.com/kubecube-io/kubecube/pkg/authorizer/rbac"
	"github.com/kubecube-io/kubecube/pkg/clog"
	userinfo "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

type Access struct {
	Cluster         string
	Name            string
	Operator        string
	APIGroup        string
	Namespace       string
	Resource        string
	ResourceRequest bool
}

func NewSimpleAccess(clusterName string, userName string, namespace string) *Access {

	return NewAccess(clusterName, userName, "get, list", "", namespace, "", true)
}

func NewAccess(clusterName string, userName string, operator string, apiGroup string, namespace string, resource string, isK8sRes bool) *Access {
	a := &Access{
		Cluster:         clusterName,
		Name:            userName,
		Operator:        operator,
		APIGroup:        apiGroup,
		Namespace:       namespace,
		Resource:        resource,
		ResourceRequest: isK8sRes,
	}
	return a
}

func (a *Access) AccessAllow(apiGroup string, resource string, operator string) bool {

	auth := authorizer.AttributesRecord{
		User:            &userinfo.DefaultInfo{Name: a.Name},
		Verb:            operator,
		APIGroup:        apiGroup,
		Namespace:       a.Namespace,
		Resource:        resource,
		ResourceRequest: true,
	}
	r := rbac.NewDefaultResolver(a.Cluster)
	d, _, err := r.Authorize(context.Background(), auth)
	if err != nil {
		clog.Error("%v", err.Error())
	}

	return d == authorizer.DecisionAllow
}
