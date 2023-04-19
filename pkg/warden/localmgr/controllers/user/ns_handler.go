package controllers

import (
	"k8s.io/apimachinery/pkg/types"
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

		users := r.toFindRelatedUsers(tenant, project)

		for _, user := range users {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: user}})
		}

		// enqueue all users we found related, and refresh them.
		return requests
	}
}

// toFindRelatedUsers will find users which belongs to given tenant or project.
func (r *UserReconciler) toFindRelatedUsers(tenant, project string) []string {
	return nil
}

// extraTenantAndProject will extra tenant and project name from given labels.
func extraTenantAndProject(ls map[string]string) (string, string) {
	return "", ""
}

func allowedPaas(ls map[string]string) bool {
	tenant, project := extraTenantAndProject(ls)
	if len(tenant) == 0 || len(project) == 0 {
		return false
	}
	// allowed paas if we got tenant and project
	return true
}
