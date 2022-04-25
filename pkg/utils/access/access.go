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

package access

import (
	"context"
	"net/http"

	userinfo "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/kubecube-io/kubecube/pkg/authentication/authenticators/token"
	"github.com/kubecube-io/kubecube/pkg/authorizer/rbac"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
)

func AllowAccess(cluster string, r *http.Request, operator string, object client.Object) bool {
	client := clients.Interface().Kubernetes(constants.LocalCluster)
	gvk, err := apiutil.GVKForObject(object, client.Direct().Scheme())
	if err != nil {
		clog.Error(err.Error())
		return false
	}
	groupKind := gvk.GroupKind()
	version := gvk.Version
	if client.RESTMapper() == nil {
		return true
	}
	mapping, err := client.RESTMapper().RESTMapping(groupKind, version)
	if err != nil {
		clog.Error(err.Error())
		return false
	}
	user, err := token.GetUserFromReq(r)
	if err != nil {
		clog.Error(err.Error())
		return false
	}
	object.GetObjectKind().GroupVersionKind()

	auth := authorizer.AttributesRecord{
		User:            &userinfo.DefaultInfo{Name: user.Username},
		Verb:            operator,
		Namespace:       object.GetNamespace(),
		APIGroup:        mapping.Resource.Group,
		APIVersion:      mapping.Resource.Version,
		Resource:        mapping.Resource.Resource,
		ResourceRequest: true,
	}
	rbac := rbac.NewDefaultResolver(cluster)
	d, _, err := rbac.Authorize(context.Background(), auth)
	if err != nil {
		clog.Error("%v", err.Error())
	}

	return d == authorizer.DecisionAllow
}

func CheckClusterRole(username string, cluster string, accessMap map[string]string) bool {
	r := rbac.NewDefaultResolver(cluster)
	user := &userinfo.DefaultInfo{Name: username}
	roleList := r.User2UserRole(user)
	for _, role := range roleList {
		if _, ok := accessMap[role]; ok {
			return true
		}
	}
	return false
}

func IsSelf(r *http.Request, username string) bool {
	requestUser, err := token.GetUserFromReq(r)
	if err != nil {
		return false

	}
	return username == requestUser.Username
}
