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

package controllers

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tenantv1 "github.com/kubecube-io/kubecube/pkg/apis/tenant/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
)

var _ reconcile.Reconciler = &TenantReconciler{}

// TenantReconciler reconciles a Tenant object
type TenantReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func newReconciler(mgr manager.Manager) (*TenantReconciler, error) {
	r := &TenantReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	return r, nil
}

//+kubebuilder:rbac:groups=tenant.kubecube.io,resources=tenants,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tenant.kubecube.io,resources=tenants/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tenant.kubecube.io,resources=tenants/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the Tenant object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.0/pkg/reconcile
func (r *TenantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := clog.WithName("reconcile").WithValues("tenant", req.NamespacedName)

	// get tenant info
	tenant := tenantv1.Tenant{}
	err := r.Client.Get(ctx, req.NamespacedName, &tenant)
	if err != nil {
		log.Warn("get tenant fail, %v", err)
		return ctrl.Result{}, nil
	}

	// if .spec.namespace not equal the standard name
	nsName := constants.TenantNsPrefix + req.Name
	if tenant.Spec.Namespace != nsName {
		tenant.Spec.Namespace = nsName
		err = r.Client.Update(ctx, &tenant)
		if err != nil {
			log.Error("update tenant .spec.namespace fail, %v", err)
			return ctrl.Result{}, err
		}
	}

	// if annotation not content kubecube.io/sync, add it
	ano := tenant.Annotations
	if ano == nil {
		ano = make(map[string]string)
	}
	if _, ok := ano["kubecube.io/sync"]; !ok {
		ano["kubecube.io/sync"] = "1"
		tenant.Annotations = ano
		err = r.Client.Update(ctx, &tenant)
		if err != nil {
			log.Error("update tenant .metadata.annotations fail, %v", err)
			return ctrl.Result{}, err
		}
	}

	// weather namespace exist, create one
	namespace := corev1.Namespace{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: nsName}, &namespace)
	if err != nil {
		if !errors.IsNotFound(err) {
			log.Warn("get tenant namespaces fail, %v", err)
			return ctrl.Result{}, err
		}
		namespace := corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Namespace",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: nsName,
				Annotations: map[string]string{
					"kubecube.io/sync": "1",
					"hnc.x-k8s.io/ns":  "true",
				},
			},
		}
		err = r.Client.Create(ctx, &namespace)
		if err != nil {
			log.Warn("create tenant namespaces fail, %v", err)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func SetupWithManager(mgr ctrl.Manager) error {
	r, err := newReconciler(mgr)
	if err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&tenantv1.Tenant{}).
		Complete(r)
}
