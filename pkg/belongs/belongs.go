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

package belongs

import (
	"strings"

	v1 "github.com/kubecube-io/kubecube/pkg/apis/user/v1"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type JudgementFunc func(user *v1.User, obj runtime.Object) (bool, error)

// not thread safe
var resourcesHandlers = map[schema.GroupVersionResource]JudgementFunc{
	{Version: "v1", Resource: constants.ResourceNamespaces}: namespaceJudgement,
	{Version: "v1", Resource: constants.ResourceNode}:       nodeJudgment,
}

func RegisterDeterminer(gvr schema.GroupVersionResource, fn JudgementFunc) {
	resourcesHandlers[gvr] = fn
}

func GetDeterminer(gvr schema.GroupVersionResource) JudgementFunc {
	return resourcesHandlers[gvr]
}

func nodeJudgment(user *v1.User, obj runtime.Object) (bool, error) {
	if v1.IsPlatformAdmin(user) {
		return true, nil
	}

	meatObj, err := meta.Accessor(obj)
	if err != nil {
		return false, err
	}

	if meatObj.GetLabels() == nil {
		return false, nil
	}

	NodeTenant, ok := meatObj.GetLabels()[constants.LabelNodeTenant]
	if ok {
		if NodeTenant == constants.ValueNodeShare {
			return true, nil
		}
		if v1.BelongsToTenant(user, NodeTenant) {
			return true, nil
		}
	}

	return false, nil
}

func namespaceJudgement(user *v1.User, obj runtime.Object) (bool, error) {
	if v1.IsPlatformAdmin(user) {
		return true, nil
	}

	meatObj, err := meta.Accessor(obj)
	if err != nil {
		return false, err
	}

	if meatObj.GetLabels() == nil {
		return false, nil
	}

	nsBelongTenant, ok := meatObj.GetLabels()[constants.HncTenantLabel]
	if ok && v1.BelongsToTenant(user, nsBelongTenant) {
		return true, nil
	}

	nsBelongProject, ok := meatObj.GetLabels()[constants.HncProjectLabel]
	if ok && v1.BelongsToProject(user, nsBelongProject) {
		return true, nil
	}

	if strings.HasPrefix(meatObj.GetName(), constants.TenantNsPrefix) {
		nsBelongTenant = strings.TrimPrefix(meatObj.GetName(), constants.TenantNsPrefix)
		if v1.BelongsToTenant(user, nsBelongTenant) {
			return true, nil
		}
	}

	return false, nil
}
