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

package cube

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	quotav1 "github.com/kubecube-io/kubecube/pkg/apis/quota/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/quota"
	"github.com/kubecube-io/kubecube/pkg/utils/strslice"
)

func isExceedParent(current, old, parent *quotav1.CubeResourceQuota) (bool, string) {
	for _, rs := range quota.ResourceNames {
		pHard := parent.Spec.Hard
		pUsed := parent.Status.Used
		cHard := current.Spec.Hard

		parentHard, ok := pHard[rs]
		if !ok {
			// if this resource kind not parent quota hard but in current quota
			// hard we consider the current quota is exceed parent limit
			if _, ok := cHard[rs]; ok {
				return true, fmt.Sprintf("can not set a resource(%v) that parent quota hard not had", rs)
			}
			// both quota have no that resource kind, continue directly
			continue
		}

		// used certainly exist if hard has
		parentUsed, ok := pUsed[rs]
		if !ok {
			if _, ok := cHard[rs]; ok {
				return true, fmt.Sprintf("can not set a resource(%v) that parent quota used not had", rs)
			}
			continue
		}

		// if this resource kind parent quota hard has but current quota has not
		// we allowed this action.
		currentHard, ok := cHard[rs]
		if !ok {
			clog.Warn("less resource(%v) but parent quota had", rs)
			return false, ""
		}

		oldHard := ensureValue(old, rs)

		// if changed > left, we consider the current quota is exceed parent limit
		changed := currentHard.DeepCopy()
		changed.Sub(oldHard)

		if isExceed(parentHard, parentUsed, changed) {
			return true, fmt.Sprintf("overload, resource(%v), parent hard(%v), parent used(%v), changed(%v)", rs, parentHard.String(), parentUsed.String(), changed.String())
		}
	}

	return false, ""
}

func refreshUsedResource(current, old, parent *quotav1.CubeResourceQuota, cli client.Client) (*quotav1.CubeResourceQuota, error) {
	newParentUsed := quota.ClearQuotas(parent.Status.Used)

	for _, sub := range parent.Status.SubResourceQuotas {
		subResourceQuota, name, err := getCubeResourceQuota(cli, sub)
		if err != nil {
			if !errors.IsNotFound(err) {
				return parent, err
			}
			needRemoveSub := true
			// use new ResourceQuota if present
			if current != nil {
				if name == current.Name {
					clog.Debug("handle current subResourceQuota %v", sub)
					subResourceQuota = current
					needRemoveSub = false
				}
			}
			if needRemoveSub {
				// remove not found subResourceQuota
				clog.Info("remove not exist subResourceQuota %v", sub)
				parent.Status.SubResourceQuotas = strslice.RemoveString(parent.Status.SubResourceQuotas, sub)
				continue
			}
		}
		// use new CubeResourceQuota if present
		if current != nil && name == current.Name {
			subResourceQuota = current
		}

		clog.Info("populate used of CubeResourceQuota %v with subResourceQuota %v", parent.Name, sub)

		for _, rs := range quota.ResourceNames {
			// continue if parent used quota had no that resource
			newUsed, ok := newParentUsed[rs]
			if !ok {
				continue
			}
			rq, ok := subResourceQuota.Spec.Hard[rs]
			if !ok {
				// continue if subResourceQuota had no that resource
				continue
			}
			newUsed.Add(rq)
			newParentUsed[rs] = newUsed
		}
	}

	parent.Status.Used = newParentUsed
	clog.Info("refreshed sub resource quota of %v is %v", parent.Name, parent.Status.SubResourceQuotas)
	clog.Debug("refreshed used of CubeResourceQuota %v is %v", parent.Name, newParentUsed)

	return parent, nil
}

func getCubeResourceQuota(cli client.Client, s string) (*quotav1.CubeResourceQuota, string, error) {
	splitS := strings.Split(s, ".")
	splitSLen := len(splitS)
	if splitSLen < 2 {
		return nil, "", fmt.Errorf("subResourceQuota name invilde: %v", s)
	}

	names := splitS[:splitSLen-1]
	name := ""
	for i, v := range names {
		if i == len(names)-1 {
			name += v
		} else {
			name += v + "."
		}
	}

	rq := &quotav1.CubeResourceQuota{}
	err := cli.Get(context.Background(), types.NamespacedName{Name: name}, rq)
	if err != nil {
		return nil, name, err
	}

	return rq, name, nil
}

func ensureValue(c *quotav1.CubeResourceQuota, key v1.ResourceName) resource.Quantity {
	if c == nil {
		return quota.ZeroQ()
	}
	oHard := c.Spec.Hard
	_, ok := oHard[key]
	if !ok {
		oHard[key] = quota.ZeroQ()
	}
	return oHard[key]

}

func isExceed(parentHard, parentUsed, changed resource.Quantity) bool {
	parentUsed.Add(changed)

	if parentUsed.Cmp(parentHard) == 1 {
		return true
	}

	if parentUsed.Cmp(quota.ZeroQ()) == -1 {
		return true
	}

	return false
}
