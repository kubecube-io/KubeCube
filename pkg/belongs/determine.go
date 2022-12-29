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
	"context"
	"fmt"

	"github.com/kubecube-io/kubecube/pkg/multicluster/client"
	v12 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/kubecube-io/kubecube/pkg/apis/user/v1"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/path"
)

func RelationshipDetermine(ctx context.Context, cli client.Client, k8sPath string, userName string) (bool, error) {
	ri, err := path.Parse(k8sPath)
	if err != nil {
		return true, fmt.Errorf("parse request url %v failed %v", k8sPath, err)
	}

	if len(ri.Name) == 0 {
		return true, fmt.Errorf("list request %v will be pass through", k8sPath)
	}

	determiner := GetDeterminer(ri.Gvr)
	if determiner != nil {
		user := &v1.User{}
		err := cli.Cache().Get(ctx, types.NamespacedName{Name: userName}, user)
		if err != nil {
			return true, err
		}
		if ri.Gvr.Resource == constants.ResourceNamespaces {
			obj := &v12.Namespace{}
			err = cli.Cache().Get(ctx, types.NamespacedName{Name: ri.Name}, obj)
			if err != nil {
				return true, err
			}
			return determiner(user, obj)
		} else if ri.Gvr.Resource == constants.ResourceNode {
			obj := &v12.Node{}
			err = cli.Cache().Get(ctx, types.NamespacedName{Name: ri.Name}, obj)
			if err != nil {
				return true, err
			}
			return determiner(user, obj)
		}
	}
	return true, nil
}
