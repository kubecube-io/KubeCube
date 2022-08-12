/*
Copyright 2022 KubeCube Authors

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

package service

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/service"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/filter"
)

var _ reconcile.Reconciler = &ServiceReconciler{}

// ServiceReconciler reconciles a Tenant object
type ServiceReconciler struct {
	*NginxConfig
	client.Client
}

func newReconciler(mgr manager.Manager, opts *NginxConfig) (*ServiceReconciler, error) {
	r := &ServiceReconciler{
		Client:      mgr.GetClient(),
		NginxConfig: opts,
	}
	return r, nil
}

func (s *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	service := v1.Service{}
	err := s.Get(ctx, req.NamespacedName, &service)
	if errors.IsNotFound(err) {
		return s.DeleteExternalAccess(ctx, req.NamespacedName)
	}
	return s.UpdateExternalAccess(ctx, req.NamespacedName, &service)
}

func (s *ServiceReconciler) UpdateExternalAccess(ctx context.Context, namespaceName types.NamespacedName, svc *v1.Service) (ctrl.Result, error) {
	clog.Debug("start update service external access: %+v", namespaceName)
	externalHandler := service.NewExternalAccess(s.Client, namespaceName.Namespace, namespaceName.Name, filter.Filter{Limit: 10}, s.NginxNamespace, s.NginxTcpServiceConfigMap, s.NginxUdpServiceConfigMap)
	oldInfo, err := externalHandler.GetExternalAccess()
	if err != nil {
		clog.Error("get external access info fail, %+v", err)
		return ctrl.Result{}, err
	}
	//if no need to update
	if len(oldInfo) == 0 {
		return ctrl.Result{}, nil
	}
	for i, es := range oldInfo {
		for _, items := range svc.Spec.Ports {
			if string(items.Protocol) == es.Protocol {
				oldInfo[i].ServicePort = int(items.Port)
			}
		}
	}
	err = externalHandler.SetExternalAccess(oldInfo)
	if err != nil {
		clog.Debug("update service external access error, %+v", err)
	}
	return ctrl.Result{}, nil

}

func (s *ServiceReconciler) DeleteExternalAccess(ctx context.Context, namespaceName types.NamespacedName) (ctrl.Result, error) {
	clog.Debug("start delete service external access: %+v", namespaceName)
	externalHandler := service.NewExternalAccess(s.Client, namespaceName.Namespace, namespaceName.Name, filter.Filter{Limit: 10}, s.NginxNamespace, s.NginxTcpServiceConfigMap, s.NginxUdpServiceConfigMap)
	err := externalHandler.DeleteExternalAccess()
	if err != nil {
		clog.Debug("delete service external access error, %+v", err)
	}
	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func SetupWithManager(mgr ctrl.Manager, opts *NginxConfig) error {
	r, err := newReconciler(mgr, opts)
	if err != nil {
		return err
	}
	predicateFunc := predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			return true
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return true
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Service{}).
		WithEventFilter(predicateFunc).
		Complete(r)
}
