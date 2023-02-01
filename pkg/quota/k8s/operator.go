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

package k8s

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	quotav1 "github.com/kubecube-io/kubecube/pkg/apis/quota/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/quota"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/strslice"
)

type QuotaOperator struct {
	PivotClient  client.Client
	LocalClient  client.Client
	CurrentQuota *v1.ResourceQuota
	OldQuota     *v1.ResourceQuota

	context.Context
}

func NewQuotaOperator(pivot, local client.Client, current, old *v1.ResourceQuota, ctx context.Context) quota.Interface {
	return &QuotaOperator{
		PivotClient:  pivot,
		LocalClient:  local,
		CurrentQuota: current,
		OldQuota:     old,
		Context:      ctx,
	}
}

func (o *QuotaOperator) Parent() (*quotav1.CubeResourceQuota, error) {
	var (
		parentName string
		ok         bool
		quotaName  string
		quotaNs    string
	)

	if o.CurrentQuota == nil {
		parentName, ok = o.OldQuota.Labels[constants.CubeQuotaLabel]
		quotaName, quotaNs = o.OldQuota.Name, o.OldQuota.Namespace
	} else {
		parentName, ok = o.CurrentQuota.Labels[constants.CubeQuotaLabel]
		quotaName, quotaNs = o.CurrentQuota.Name, o.CurrentQuota.Namespace
	}

	if !ok {
		clog.Warn("resourceQuota (%v/%v) without cube quota label: kubecube.io/quota", quotaNs, quotaName)
		return nil, nil
	}

	if parentName == "" {
		return nil, nil
	}

	key := types.NamespacedName{Name: parentName}
	parentQuota := &quotav1.CubeResourceQuota{}

	err := o.PivotClient.Get(o.Context, key, parentQuota)
	if err != nil {
		return nil, err
	}

	return parentQuota, nil
}

func (o *QuotaOperator) Overload() (bool, string, error) {
	parentQuota, err := o.Parent()
	if err == nil && parentQuota == nil {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}

	isOverload, reason := o.isExceedParent(parentQuota)

	return isOverload, reason, nil
}

func (o *QuotaOperator) UpdateParentStatus(flush bool) error {
	parentQuota, err := o.Parent()
	if err != nil {
		return err
	}

	if parentQuota == nil {
		return nil
	}

	currentQuota := o.CurrentQuota.DeepCopy()
	oldQuota := o.OldQuota.DeepCopy()

	// update subResourceQuotas status of parent
	var subResourceQuota string
	if currentQuota != nil {
		subResourceQuota = fmt.Sprintf("%v.%v.%v", currentQuota.Name, currentQuota.Namespace, quota.SubFix)
	}
	if oldQuota != nil {
		subResourceQuota = fmt.Sprintf("%v.%v.%v", oldQuota.Name, oldQuota.Namespace, quota.SubFix)
	}

	// populate new sub resource quotas
	switch flush {
	case true:
		subResourceQuotas := parentQuota.Status.SubResourceQuotas
		if subResourceQuotas != nil {
			parentQuota.Status.SubResourceQuotas = strslice.RemoveString(subResourceQuotas, subResourceQuota)
		}
	case false:
		if parentQuota.Status.SubResourceQuotas == nil {
			parentQuota.Status.SubResourceQuotas = []string{subResourceQuota}
		} else {
			parentQuota.Status.SubResourceQuotas = strslice.InsertString(parentQuota.Status.SubResourceQuotas, subResourceQuota)
		}
	}

	// refresh new used of parent quota
	refreshed, err := refreshUsedResource(currentQuota, oldQuota, parentQuota, o.LocalClient)
	if err != nil {
		return err
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		newQuota := &quotav1.CubeResourceQuota{}
		err := o.PivotClient.Get(context.Background(), types.NamespacedName{Name: refreshed.Name}, newQuota)
		if err != nil {
			return err
		}
		newQuota.Status = refreshed.Status
		err = o.PivotClient.Status().Update(o.Context, newQuota)
		if err != nil {
			return err
		}
		return nil
	})
}

func IsRelyOnObj(quotas ...*v1.ResourceQuota) bool {
	for _, q := range quotas {
		if q != nil {
			if len(q.UID) > 0 {
				return true
			}
		}
	}
	return false
}
