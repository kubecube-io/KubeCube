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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/kubecube-io/kubecube/pkg/clog"

	quotav1 "github.com/kubecube-io/kubecube/pkg/apis/quota/v1"
	"github.com/kubecube-io/kubecube/pkg/quota"
)

func isExceedParent(current, old, parent *quotav1.CubeResourceQuota) bool {
	for _, rs := range quota.ResourceNames {
		pHard := parent.Spec.Hard
		pUsed := parent.Status.Used
		cHard := current.Spec.Hard

		parentHard, ok := pHard[rs]
		if !ok {
			// if this resource kind not parent quota hard but in current quota
			// hard we consider the current quota is exceed parent limit
			if _, ok := cHard[rs]; ok {
				return true
			}
			// both quota have no that resource kind, continue directly
			continue
		}

		// used certainly exist if hard has
		parentUsed, ok := pUsed[rs]
		if !ok {
			if _, ok := cHard[rs]; ok {
				return true
			}
			continue
		}

		// if this resource kind parent quota hard has but current quota has not
		// we consider the current quota is exceed parent limit
		currentHard, ok := cHard[rs]
		if !ok {
			return true
		}

		oldHard := ensureValue(old, rs)

		// if changed > left, we consider the current quota is exceed parent limit
		changed := currentHard.DeepCopy()
		changed.Sub(oldHard)
		if isExceed(parentHard, parentUsed, changed) {
			clog.Debug("overload, resource: %v, parent hard: %v, parent used: %v, changed: %v", rs, parentHard.String(), parentUsed.String(), changed.String())
			return true
		}
	}

	return false
}

func refreshUsedResource(current, old, parent *quotav1.CubeResourceQuota) *quotav1.CubeResourceQuota {
	for _, rs := range quota.ResourceNames {
		pUsed := parent.Status.Used
		pHard := parent.Spec.Hard

		// parent used
		parentUsed, ok := pUsed[rs]
		if !ok {
			continue
		}

		// hard certainly exist if hard has
		// parent used
		parentHard := pHard[rs]

		oldHard := ensureValue(old, rs)
		currentHard := ensureValue(current, rs)

		// newUsed = newHard - oldHard + oldUsed
		changed := currentHard.DeepCopy()
		changed.Sub(oldHard)

		newUsed := parentUsed.DeepCopy()
		newUsed.Add(changed)

		if newUsed.Cmp(parentHard) == 1 {
			clog.Error("quota new used bigger than hard of parent %v, new used: %v, hard: %v", parent.Name, newUsed.String(), parentHard.String())
		}

		parent.Status.Used[rs] = newUsed
	}

	return parent
}

func ensureValue(c *quotav1.CubeResourceQuota, key v1.ResourceName) resource.Quantity {
	q := resource.Quantity{}
	if c == nil {
		q = quota.ZeroQ()
	} else {
		oHard := c.Spec.Hard
		_, ok := oHard[key]
		if !ok {
			oHard[key] = quota.ZeroQ()
		}
		q = oHard[key]
	}

	return q
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
