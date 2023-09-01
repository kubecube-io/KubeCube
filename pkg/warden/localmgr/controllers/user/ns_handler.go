package controllers

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"

	userv1 "github.com/kubecube-io/kubecube/pkg/apis/user/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var namespacePredicateFn = builder.WithPredicates(predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool {
		return allowedPaas(e.Object.GetLabels())
	},
	UpdateFunc: func(e event.UpdateEvent) bool {
		return false
	},
	DeleteFunc: func(event.DeleteEvent) bool {
		return false
	},
	GenericFunc: func(event.GenericEvent) bool {
		return false
	},
})

func (r *UserReconciler) namespaceHandleFunc() handler.MapFunc {
	return func(obj client.Object) []reconcile.Request {
		var requests []reconcile.Request

		// paas filter it sure there must be tenant and project
		tenant, project := extraTenantAndProject(obj.GetLabels())

		users, err := r.toFindRelatedUsers(tenant, project)
		if err != nil {
			clog.Error("find related users failed, tenant: (%v), project: (%v), err: %v", tenant, project, err)
			return requests
		}

		for _, user := range users {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: user}})
		}

		// enqueue all users we found related, and refresh them.
		return requests
	}
}

// toFindRelatedUsers will find users which belongs to given tenant or project.
func (r *UserReconciler) toFindRelatedUsers(tenant, project string) ([]string, error) {
	userList := userv1.UserList{}
	err := r.List(context.TODO(), &userList)
	if err != nil {
		return nil, err
	}

	relatedUsers := []string{}

	for _, user := range userList.Items {
		tenantSet := sets.NewString(user.Status.BelongTenants...)
		projectSet := sets.NewString(user.Status.BelongProjects...)
		if tenantSet.Has(tenant) || projectSet.Has(project) {
			relatedUsers = append(relatedUsers, user.Name)
		}
	}

	return relatedUsers, nil
}

// extraTenantAndProject will extra tenant and project name from given labels.
func extraTenantAndProject(ls map[string]string) (string, string) {
	if ls == nil {
		return "", ""
	}
	return ls[constants.HncTenantLabel], ls[constants.HncProjectLabel]
}

func allowedPaas(ls map[string]string) bool {
	tenant, project := extraTenantAndProject(ls)
	if len(tenant) == 0 || len(project) == 0 {
		return false
	}
	// allowed paas if we got tenant and project
	return true
}
