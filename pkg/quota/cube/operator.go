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

	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"k8s.io/client-go/util/retry"

	"github.com/kubecube-io/kubecube/pkg/utils/strslice"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubecube-io/kubecube/pkg/quota"

	quotav1 "github.com/kubecube-io/kubecube/pkg/apis/quota/v1"
	"k8s.io/apimachinery/pkg/types"
)

type QuotaOperator struct {
	Client       client.Client
	CurrentQuota *quotav1.CubeResourceQuota
	OldQuota     *quotav1.CubeResourceQuota

	context.Context
}

func NewQuotaOperator(client client.Client, current, old *quotav1.CubeResourceQuota, ctx context.Context) quota.Interface {
	return &QuotaOperator{
		Client:       client,
		CurrentQuota: current,
		OldQuota:     old,
		Context:      ctx,
	}
}

func (o *QuotaOperator) Parent() (*quotav1.CubeResourceQuota, error) {
	var parentName string

	if o.CurrentQuota == nil {
		parentName = o.OldQuota.Spec.ParentQuota
	} else {
		parentName = o.CurrentQuota.Spec.ParentQuota
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

func (o *QuotaOperator) Overload() (bool, string, error) {
	currentQuota := o.CurrentQuota
	oldQuota := o.OldQuota

	// todo: there is must be a way limit the hard of node pool kind
	if isTenantKind(currentQuota, oldQuota) {
		return false, "", nil
	}

	parentQuota, err := o.Parent()
	if err != nil || parentQuota == nil {
		return false, "", err
	}

	isOverload, reason := isExceedParent(currentQuota, oldQuota, parentQuota)

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
		subResourceQuota = fmt.Sprintf("%v.%v", currentQuota.Name, quota.SubFix)
	}
	if oldQuota != nil {
		subResourceQuota = fmt.Sprintf("%v.%v", oldQuota.Name, quota.SubFix)
	}

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

	// update used status of parent
	refreshed, err := refreshUsedResource(currentQuota, oldQuota, parentQuota, o.Client)
	if err != nil {
		return err
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		newQuota := &quotav1.CubeResourceQuota{}
		err := o.Client.Get(context.Background(), types.NamespacedName{Name: refreshed.Name}, newQuota)
		if err != nil {
			return err
		}
		newQuota.Status = refreshed.Status
		err = o.Client.Status().Update(o.Context, newQuota)
		if err != nil {
			return err
		}
		return nil
	})
}

func isTenantKind(quotas ...*quotav1.CubeResourceQuota) bool {
	for _, q := range quotas {
		if q != nil {
			if q.Spec.Target.Kind == quotav1.TenantObj {
				return true
			}
		}
	}

	return false
}

// InitStatus initialize status of quota
func InitStatus(current *quotav1.CubeResourceQuota) {
	current.Status.Hard = current.Spec.Hard
	// if target object of quota is NodesPool, we should use the physical resource
	// as value to the hard of the status
	//if current.Spec.Target.Kind == quotav1.NodesPoolObj {
	//	current.Status.Hard = physicalResourceFrom(current.Spec.Target.Name)
	//}

	used := make(map[v1.ResourceName]resource.Quantity)
	for k := range current.Spec.Hard {
		used[k] = quota.ZeroQ()
	}

	current.Status.Used = used
	current.Status.SubResourceQuotas = make([]string, 0)
}

// todo: get real physical resource from nodes pool cr, use cluster name at begin
func physicalResourceFrom(objName string) v1.ResourceList {
	cli := clients.Interface().Kubernetes(objName)
	nodes := v1.NodeList{}
	err := cli.Cache().List(context.Background(), &nodes)
	if err != nil {
		clog.Error("get cluster %v nodes failed: %v", objName, err)
	}

	cpu := quota.ZeroQ()
	mem := quota.ZeroQ()
	storage := quota.ZeroQ()
	storageEphemeral := quota.ZeroQ()
	pods := quota.ZeroQ()

	for _, v := range nodes.Items {
		cpu.Add(*v.Status.Capacity.Cpu())
		mem.Add(*v.Status.Capacity.Memory())
		storage.Add(*v.Status.Capacity.Storage())
		storageEphemeral.Add(*v.Status.Capacity.StorageEphemeral())
		pods.Add(*v.Status.Capacity.Pods())
	}

	r := map[v1.ResourceName]resource.Quantity{
		v1.ResourceRequestsCPU:              cpu,
		v1.ResourceLimitsCPU:                cpu,
		v1.ResourceCPU:                      cpu,
		v1.ResourceRequestsMemory:           mem,
		v1.ResourceLimitsMemory:             mem,
		v1.ResourceMemory:                   mem,
		v1.ResourceRequestsEphemeralStorage: storageEphemeral,
		v1.ResourceLimitsEphemeralStorage:   storageEphemeral,
		v1.ResourceEphemeralStorage:         storageEphemeral,
		v1.ResourceStorage:                  storage,
		v1.ResourcePods:                     pods,
	}

	return r
}

// AllowedDel return true if deletion of current cube resource quota
// is allowed, otherwise false
func AllowedDel(current *quotav1.CubeResourceQuota) bool {
	if current.Status.SubResourceQuotas != nil {
		if len(current.Status.SubResourceQuotas) > 0 {
			return false
		}
	}

	return true
}

// AllowedUpdate return false if hard of current is less than old status
// otherwise true
func AllowedUpdate(current, old *quotav1.CubeResourceQuota) bool {
	for _, rs := range quota.ResourceNames {
		currentHard := current.Spec.Hard
		oldUsed := old.Status.Used

		cHard, ok := currentHard[rs]
		if !ok {
			// if resource not in current but in old used we thought
			// its not allowed update
			_, ok = oldUsed[rs]
			if ok {
				return false
			}
		}

		oUsed, ok := oldUsed[rs]
		if !ok {
			continue
		}

		if cHard.Cmp(oUsed) == -1 {
			return false
		}
	}

	return true
}

func IsRelyOnObj(quotas ...*quotav1.CubeResourceQuota) bool {
	for _, q := range quotas {
		if q != nil {
			if len(q.UID) > 0 {
				return true
			}
		}
	}
	return false
}
