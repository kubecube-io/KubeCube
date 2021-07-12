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
	"errors"
	"fmt"

	"k8s.io/client-go/util/retry"

	"github.com/kubecube-io/kubecube/pkg/utils/strslice"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubecube-io/kubecube/pkg/quota"

	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"k8s.io/apimachinery/pkg/types"

	quotav1 "github.com/kubecube-io/kubecube/pkg/apis/quota/v1"
	v1 "k8s.io/api/core/v1"
)

type QuotaOperator struct {
	Client       client.Client
	CurrentQuota *v1.ResourceQuota
	OldQuota     *v1.ResourceQuota

	context.Context
}

func NewQuotaOperator(client client.Client, current, old *v1.ResourceQuota, ctx context.Context) quota.Interface {
	return &QuotaOperator{
		Client:       client,
		CurrentQuota: current,
		OldQuota:     old,
		Context:      ctx,
	}
}

func (o *QuotaOperator) Parent() (*quotav1.CubeResourceQuota, error) {
	var (
		parentName string
		ok         bool
	)

	if o.CurrentQuota == nil {
		parentName, ok = o.OldQuota.Labels[constants.CubeQuotaLabel]
	} else {
		parentName, ok = o.CurrentQuota.Labels[constants.CubeQuotaLabel]
	}

	if !ok {
		return nil, errors.New("resourceQuota without cube quota label: kubecube.io/quota")
	}

	if parentName == "" {
		return nil, nil
	}

	key := types.NamespacedName{Name: parentName}
	parentQuota := &quotav1.CubeResourceQuota{}

	err := o.Client.Get(o.Context, key, parentQuota)
	if err != nil {
		return nil, err
	}

	return parentQuota, nil
}

func (o *QuotaOperator) Overload() (bool, error) {
	currentQuota := o.CurrentQuota
	oldQuota := o.OldQuota

	parentQuota, err := o.Parent()
	if err != nil || parentQuota == nil {
		return false, err
	}

	return isExceedParent(currentQuota, oldQuota, parentQuota), nil
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

	refreshed := refreshUsedResource(currentQuota, oldQuota, parentQuota)

	// update subResourceQuotas status of parent
	var subResourceQuota string
	if currentQuota != nil {
		subResourceQuota = fmt.Sprintf("%v.%v.%v", currentQuota.Name, currentQuota.Namespace, quota.SubFix)
	}
	if oldQuota != nil {
		subResourceQuota = fmt.Sprintf("%v.%v.%v", oldQuota.Name, oldQuota.Namespace, quota.SubFix)
	}

	switch flush {
	case true:
		subResourceQuotas := refreshed.Status.SubResourceQuotas
		if subResourceQuotas != nil {
			refreshed.Status.SubResourceQuotas = strslice.RemoveString(subResourceQuotas, subResourceQuota)
		}
	case false:
		if refreshed.Status.SubResourceQuotas == nil {
			refreshed.Status.SubResourceQuotas = []string{subResourceQuota}
		} else {
			refreshed.Status.SubResourceQuotas = strslice.InsertString(refreshed.Status.SubResourceQuotas, subResourceQuota)
		}
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err = o.Client.Status().Update(o.Context, refreshed)
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
