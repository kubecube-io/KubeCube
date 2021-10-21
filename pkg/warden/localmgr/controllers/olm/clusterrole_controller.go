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

package olm

import (
	"context"
	"strings"

	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	log clog.CubeLogger

	_ reconcile.Reconciler = &ClusterRoleReconciler{}
)

type ClusterRoleReconciler struct {
	client.Client
}

func newReconciler(mgr manager.Manager) (*ClusterRoleReconciler, error) {
	log = clog.WithName("OperatorGroup")

	r := &ClusterRoleReconciler{
		Client: mgr.GetClient(),
	}

	return r, nil
}

// Reconcile that reconcile OperatorGroup event but write nothing
// to OperatorGroup cr, all things we do with OG are read-only
func (r *ClusterRoleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log.Info("Reconcile OG ClusterRole (%v)", req.Name)

	ogClusterRole := &rbacv1.ClusterRole{}

	err := r.Client.Get(ctx, req.NamespacedName, ogClusterRole)
	if err != nil {
		if errors.IsNotFound(err) {
			// try to delete related ClusterRole when ogClusterRole was deleted
			return ctrl.Result{}, r.tryDeleteClusterRole(req)
		}
		return ctrl.Result{}, err
	}

	return r.sync(ogClusterRole)
}

func (r *ClusterRoleReconciler) sync(ogClusterRole *rbacv1.ClusterRole) (ctrl.Result, error) {
	// aggregate og ClusterRole to platform-admin, tenant-admin, project-admin
	labels := map[string]string{
		constants.PlatformAdminAgLabel: "true",
		constants.TenantAdminAgLabel:   "true",
		constants.ProjectAdminAgLabel:  "true",
	}

	ogClusterRoleMirror := &rbacv1.ClusterRole{
		// name sample: mirror.og-single-admin
		ObjectMeta: metav1.ObjectMeta{Name: nameFromOg(ogClusterRole.Name), Labels: labels},
		Rules:      ogClusterRole.Rules,
	}

	// create og ClusterRole mirror to make sense
	return ctrl.Result{}, r.createOrUpdateClusterRole(ogClusterRoleMirror)
}

func (r *ClusterRoleReconciler) tryDeleteClusterRole(req ctrl.Request) error {
	obj := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: nameFromOg(req.Name)}}
	err := r.Delete(context.Background(), obj)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (r *ClusterRoleReconciler) createOrUpdateClusterRole(object *rbacv1.ClusterRole) error {
	objCopy := object.DeepCopy()
	res, err := controllerutil.CreateOrUpdate(context.Background(), r.Client, objCopy, func() error {
		objCopy.Labels = object.Labels
		objCopy.Annotations = object.Annotations
		objCopy.Rules = object.Rules
		objCopy.AggregationRule = object.AggregationRule
		return nil
	})
	if err != nil {
		return err
	}

	if res == controllerutil.OperationResultCreated {
		clog.Info("Create ClusterRole (%v) successed", object.GetName())
	} else if res == controllerutil.OperationResultUpdated {
		clog.Info("Update ClusterRole (%v) successed", object.GetName())
	} else {
		clog.Info("ClusterRole (%v) is up to date", object.GetName())
	}

	return nil
}

func nameFromOg(name string) string {
	return "mirror." + name
}

func isPassedFilter(obj client.Object) bool {
	v, ok := obj.GetLabels()["olm.owner.kind"]
	if !ok {
		return false
	}

	if v != "OperatorGroup" {
		return false
	}

	return strings.Contains(obj.GetName(), "-admin")
}

func SetupWithManager(mgr ctrl.Manager) error {
	r, err := newReconciler(mgr)
	if err != nil {
		return err
	}

	predicateFunc := predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			return isPassedFilter(event.Object)
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			return isPassedFilter(updateEvent.ObjectOld) && isPassedFilter(updateEvent.ObjectNew)
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return isPassedFilter(deleteEvent.Object)
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	}

	return ctrl.NewControllerManagedBy(mgr).For(&rbacv1.ClusterRole{}).WithEventFilter(predicateFunc).Complete(r)
}
