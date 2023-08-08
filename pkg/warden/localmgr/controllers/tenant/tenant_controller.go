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
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/kubecube-io/kubecube/pkg/apis/quota/v1"
	tenantv1 "github.com/kubecube-io/kubecube/pkg/apis/tenant/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/env"
)

var _ reconcile.Reconciler = &TenantReconciler{}

const (
	// Default timeouts to be used in TimeoutContext
	waitInterval = 2 * time.Second
	waitTimeout  = 120 * time.Second
)

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
		if errors.IsNotFound(err) {
			return r.deleteTenant(req.Name)
		}
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
	if _, ok := ano[constants.SyncAnnotation]; !ok {
		ano[constants.SyncAnnotation] = "1"
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
		newNamespace := corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Namespace",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: nsName,
				Annotations: map[string]string{
					constants.SyncAnnotation: "1",
					"hnc.x-k8s.io/ns":        "true", // todo: deprecated annotation
				},
				Labels: env.EnsureManagedLabels(env.HncManagedLabels),
			},
		}
		err = r.Client.Create(ctx, &newNamespace)
		if err != nil {
			log.Warn("create tenant namespaces fail, %v", err)
			return ctrl.Result{}, err
		}
	}

	if namespace.Labels == nil {
		return ctrl.Result{}, nil
	}

	// ensure namespace has hnc managed labels
	needUpdate := false
	for k, v := range env.HncManagedLabels {
		if _, ok := namespace.Labels[k]; ok && v == "-" {
			delete(namespace.Labels, k)
			needUpdate = true
			continue
		}
		if namespace.Labels[k] != v {
			namespace.Labels[k] = v
			needUpdate = true
		}
	}
	if needUpdate {
		err = r.Client.Update(ctx, &namespace, &client.UpdateOptions{})
		if err != nil {
			log.Warn("update tenant namespace %v labels failed: $v", namespace.Name, err)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}
func (r *TenantReconciler) deleteTenant(tenantName string) (ctrl.Result, error) {
	// get projects in tenant
	// delete namespace of tenant
	if err := r.deleteNSofTenant(tenantName); err != nil {
		return ctrl.Result{}, err
	}
	// delete cubeResourceQuota of tenant
	err := r.deleteCubeResourceQuotaOfTenant(tenantName)
	return ctrl.Result{}, err
}

func (r *TenantReconciler) deleteNSofTenant(tenantName string) error {
	namespace := &corev1.Namespace{}
	name := constants.TenantNsPrefix + tenantName
	ctx := context.Background()
	if err := r.Client.Get(ctx, types.NamespacedName{Name: name}, namespace); err != nil {
		if errors.IsNotFound(err) {
			return nil
		} else {
			clog.Error("get namespace of tenant err: %s", err.Error())
			return fmt.Errorf("get namespace of tenant err")
		}
	}
	if err := r.Client.Delete(ctx, namespace); err != nil {
		clog.Error("delete namespace of tenant err: %s", err.Error())
		return err
	}
	err := wait.Poll(waitInterval, waitTimeout,
		func() (bool, error) {
			e := r.Client.Get(ctx, types.NamespacedName{Name: name}, namespace)
			if errors.IsNotFound(e) {
				return true, nil
			} else {
				return false, nil
			}
		})
	if err != nil {
		clog.Error("wait for delete namespace of tenant err: %s", err.Error())
		return err
	}
	return nil
}

func (r *TenantReconciler) deleteCubeResourceQuotaOfTenant(tenantName string) error {
	quota := v1.CubeResourceQuota{}
	err := r.Client.DeleteAllOf(context.TODO(), &quota, client.MatchingLabels{constants.TenantLabel: tenantName})
	if err != nil && !errors.IsNotFound(err) {
		clog.Error("delete cube resource quota error， tenant name: %s, error: %s", tenantName, err.Error())
		return err
	}
	return nil
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
