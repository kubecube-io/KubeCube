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
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/service"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/filter"
	"github.com/kubecube-io/kubecube/pkg/utils/informer"
)

type Reconciler struct {
	*NginxConfig
	client.Client
	cache    cache.Cache
	Informer cache.Informer
}

func NewServiceReconciler(mgr manager.Manager, opts *NginxConfig) (*Reconciler, error) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	c, err := cache.New(mgr.GetConfig(), cache.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}

	s := v1.Service{}
	im, err := c.GetInformer(context.Background(), &s)
	if err != nil {
		return nil, err
	}

	return &Reconciler{cache: c, Informer: im, Client: mgr.GetClient(), NginxConfig: opts}, nil
}

func (s *Reconciler) OnServiceAdd(obj interface{}) {
	return
}

func (s *Reconciler) OnServiceUpdate(oldObj, newObj interface{}) {
	oldService := oldObj.(*v1.Service)
	newService := newObj.(*v1.Service)

	clog.Info("start update service external access: %+v", oldService)
	externalHandler := service.NewExternalAccess(s.Client, oldService.Namespace, oldService.Name, filter.Filter{Limit: 10}, s.NginxNamespace, s.NginxTcpServiceConfigMap, s.NginxUdpServiceConfigMap)
	tcpInfo, udpInfo, err := externalHandler.GetExternalAccessConfigMap()
	if err != nil {
		clog.Error("get external access info fail, %+v", err)
		return
	}
	//if no need to update
	if len(tcpInfo) == 0 && len(udpInfo) == 0 {
		return
	}

	var result []service.ExternalAccessInfo
	for oldTcpServicePort, value := range tcpInfo {
		for _, servicePort := range oldService.Spec.Ports {
			// find old service port name
			if servicePort.Port == int32(oldTcpServicePort) && servicePort.Protocol == v1.ProtocolTCP {
				name := servicePort.Name
				// find new service port by name
				for _, newServicePort := range newService.Spec.Ports {
					if newServicePort.Name == name && newServicePort.Protocol == v1.ProtocolTCP {
						//update service port
						value.ServicePort = int(newServicePort.Port)
						result = append(result, *value)
						continue
					}
				}
			}
		}
	}
	for oldUdpServicePort, value := range udpInfo {
		for _, servicePort := range oldService.Spec.Ports {
			if servicePort.Port == int32(oldUdpServicePort) && servicePort.Protocol == v1.ProtocolUDP {
				name := servicePort.Name
				for _, newServicePort := range newService.Spec.Ports {
					if newServicePort.Name == name && newServicePort.Protocol == v1.ProtocolUDP {
						value.ServicePort = int(newServicePort.Port)
						result = append(result, *value)
						continue
					}
				}
			}
		}
	}
	err = externalHandler.SetExternalAccess(result)
	if err != nil {
		clog.Debug("update service external access error, %+v", err)
	}
}

func (s *Reconciler) OnServiceDelete(obj interface{}) {
	deleteService := obj.(*v1.Service)
	externalHandler := service.NewExternalAccess(s.Client, deleteService.Namespace, deleteService.Name, filter.Filter{Limit: 10}, s.NginxNamespace, s.NginxTcpServiceConfigMap, s.NginxUdpServiceConfigMap)
	err := externalHandler.DeleteExternalAccess()
	if err != nil {
		clog.Debug("delete service external access error, %+v", err)
	}
}

// Start keep sync cluster change by informer
func (s *Reconciler) Start(ctx context.Context) error {

	go func() {
		err := s.cache.Start(ctx)
		if err != nil {
			clog.Fatal("start service controller cache failed")
		}
		clog.Info("service controller exit")
	}()

	if !s.cache.WaitForCacheSync(ctx) {
		return fmt.Errorf("service controller cache can not wait for sync")
	}
	clog.Info("service controller is running")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func SetupWithManager(mgr ctrl.Manager, opts *NginxConfig) (*Reconciler, error) {
	r, err := NewServiceReconciler(mgr, opts)
	if err != nil {
		return nil, err
	}

	r.Informer.AddEventHandler(informer.NewHandlerOnEvents(r.OnServiceAdd, r.OnServiceUpdate, r.OnServiceDelete))
	return r, nil
}
