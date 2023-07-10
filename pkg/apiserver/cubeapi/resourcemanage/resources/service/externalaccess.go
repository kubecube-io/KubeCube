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

package service

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	jsoniter "github.com/json-iterator/go"
	"github.com/kubecube-io/kubecube/pkg/clog"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcemanage "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/handle"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/enum"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"github.com/kubecube-io/kubecube/pkg/utils/filter"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

const (
	TCP = "TCP"
	UDP = "UDP"
)

type ExternalAccess struct {
	ctx                      context.Context
	client                   client.Client
	namespace                string
	name                     string
	filterCondition          *filter.Condition
	NginxNamespace           string
	NginxTcpServiceConfigMap string
	NginxUdpServiceConfigMap string
}

func NewExternalAccess(client client.Client, namespace string, name string, condition *filter.Condition, nginxNs string, tcpCm string, udpCm string) ExternalAccess {
	ctx := context.Background()
	return ExternalAccess{
		ctx:                      ctx,
		client:                   client,
		namespace:                namespace,
		name:                     name,
		filterCondition:          condition,
		NginxNamespace:           nginxNs,
		NginxTcpServiceConfigMap: tcpCm,
		NginxUdpServiceConfigMap: udpCm,
	}
}

type ExternalAccessInfo struct {
	ServicePort     int    `json:"servicePort,omitempty"`
	Protocol        string `json:"protocol,omitempty"`
	ExternalPort    *int   `json:"externalPort,omitempty"`
	ServicePortName string `json:"servicePortName,omitempty"`
}

func init() {
	resourcemanage.SetExtendHandler(enum.ExternalAccessResourceType, ExternalHandle)
}

func ExternalHandle(param resourcemanage.ExtendContext) (interface{}, *errcode.ErrorInfo) {
	access := resources.NewSimpleAccess(param.Cluster, param.Username, param.Namespace)
	kubernetes := clients.Interface().Kubernetes(param.Cluster)
	if kubernetes == nil {
		return nil, errcode.ClusterNotFoundError(param.Cluster)
	}
	externalAccess := NewExternalAccess(kubernetes.Direct(), param.Namespace, param.ResourceName, param.FilterCondition, param.NginxNamespace, param.NginxTcpServiceConfigMap, param.NginxUdpServiceConfigMap)
	switch param.Action {
	case http.MethodGet:
		if allow := access.AccessAllow("", "services", "list"); !allow {
			return nil, errcode.ForbiddenErr
		}
		return externalAccess.getExternalAccess()
	case http.MethodPost:
		if allow := access.AccessAllow("", "services", "create"); !allow {
			return nil, errcode.ForbiddenErr
		}
		var externalServices []ExternalAccessInfo
		err := param.GinContext.ShouldBindJSON(&externalServices)
		if err != nil {
			return nil, errcode.InvalidBodyFormat
		}
		err = externalAccess.SetExternalAccess(externalServices)
		if err != nil {
			return nil, errcode.BadRequest(err)
		}
		return "success", nil
	default:
		return nil, errcode.InvalidHttpMethod
	}
}

func (s *ExternalAccess) SetExternalAccess(externalServices []ExternalAccessInfo) error {

	// get service
	var service v1.Service
	err := s.client.Get(s.ctx, types.NamespacedName{Namespace: s.namespace, Name: s.name}, &service)
	if err != nil {
		return err
	}
	testService := service.DeepCopy()
	testService.Name = service.Name + "-test"
	testService.ResourceVersion = ""
	testService.UID = ""
	ports := make([]v1.ServicePort, 0)
	i := 0
	for _, info := range externalServices {
		if info.ExternalPort == nil {
			continue
		}
		port := v1.ServicePort{
			Port:       int32(info.ServicePort),
			NodePort:   int32(*info.ExternalPort),
			TargetPort: intstr.FromInt(info.ServicePort),
			Name:       fmt.Sprintf("%s--%d", service.Name, i),
		}
		i++
		ports = append(ports, port)
	}
	if len(ports) != 0 {
		testService.Spec.Ports = ports
		testService.Spec.Type = v1.ServiceTypeNodePort
		err = s.client.Create(s.ctx, testService, client.DryRunAll)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			clog.Error("creaty dry service err: %s", err.Error())
			return fmt.Errorf("this externalPort is already in use")
		}
	}
	// get configmap
	var tcpcm v1.ConfigMap
	var udpcm v1.ConfigMap
	err = s.client.Get(s.ctx, types.NamespacedName{Namespace: s.NginxNamespace, Name: s.NginxTcpServiceConfigMap}, &tcpcm)
	if err != nil {
		return fmt.Errorf("can not find configmap %s in %s from cluster, %v", s.NginxTcpServiceConfigMap, s.NginxNamespace, err)
	}
	err = s.client.Get(s.ctx, types.NamespacedName{Namespace: s.NginxNamespace, Name: s.NginxUdpServiceConfigMap}, &udpcm)
	if err != nil {
		return fmt.Errorf("can not find configmap %s in %s from cluster, %v", s.NginxUdpServiceConfigMap, s.NginxNamespace, err)
	}
	if tcpcm.Data == nil {
		tcpcm.Data = make(map[string]string)
	}
	if udpcm.Data == nil {
		udpcm.Data = make(map[string]string)
	}

	// clear old
	value := fmt.Sprintf("%s/%s:", s.namespace, s.name)
	for k, v := range tcpcm.Data {
		if strings.HasPrefix(v, value) {
			delete(tcpcm.Data, k)
		}
	}
	for k, v := range udpcm.Data {
		if strings.HasPrefix(v, value) {
			delete(udpcm.Data, k)
		}
	}

	// update
	for _, es := range externalServices {
		if es.ExternalPort == nil {
			continue
		}
		// in serivce spec
		valid := false
		for _, items := range service.Spec.Ports {
			if string(items.Protocol) == es.Protocol {
				if int(items.Port) == es.ServicePort {
					valid = true
					break
				}
			}
		}
		if !valid {
			return fmt.Errorf("the service port is not exist")
		}
		dumpTest := make(map[int]int)
		ep := *es.ExternalPort
		if ep < 1 || ep > 65535 || ep == 80 || ep == 443 {
			return fmt.Errorf("the external port is invalid")
		}
		// is dump
		if _, ok := dumpTest[ep]; ok {
			return fmt.Errorf("dump external ports")
		} else {
			dumpTest[ep] = 0
		}
		// port conflict
		if _, ok := tcpcm.Data[strconv.Itoa(ep)]; ok {
			return fmt.Errorf("the external port conflict")
		}
		if _, ok := udpcm.Data[strconv.Itoa(ep)]; ok {
			return fmt.Errorf("the external port conflict")
		}
		// set port to configmap
		if strings.EqualFold(es.Protocol, TCP) {
			tcpcm.Data[strconv.Itoa(ep)] = fmt.Sprintf("%s/%s:%d", s.namespace, s.name, es.ServicePort)
		} else if strings.EqualFold(es.Protocol, UDP) {
			udpcm.Data[strconv.Itoa(ep)] = fmt.Sprintf("%s/%s:%d", s.namespace, s.name, es.ServicePort)
		} else {
			return fmt.Errorf("not support protocol")
		}
	}

	err = s.client.Update(s.ctx, &tcpcm)
	if err != nil {
		return fmt.Errorf("update fail")
	}
	err = s.client.Update(s.ctx, &udpcm)
	if err != nil {
		return fmt.Errorf("update fail")
	}
	return nil
}

func (s *ExternalAccess) GetExternalAccessConfigMap() (tcpResultMap map[int]*ExternalAccessInfo, udpResultMap map[int]*ExternalAccessInfo, err error) {
	var tcpcm v1.ConfigMap
	var udpcm v1.ConfigMap
	tcpResultMap = make(map[int]*ExternalAccessInfo)
	udpResultMap = make(map[int]*ExternalAccessInfo)
	err = s.client.Get(s.ctx, types.NamespacedName{Namespace: s.NginxNamespace, Name: s.NginxTcpServiceConfigMap}, &tcpcm)
	if err != nil {
		return tcpResultMap, udpResultMap, fmt.Errorf("can not find configmap %s in %s from cluster, %v", s.NginxTcpServiceConfigMap, s.NginxNamespace, err)
	}
	err = s.client.Get(s.ctx, types.NamespacedName{Namespace: s.NginxNamespace, Name: s.NginxUdpServiceConfigMap}, &udpcm)
	if err != nil {
		return tcpResultMap, udpResultMap, fmt.Errorf("can not find configmap %s in %s from cluster, %v", s.NginxUdpServiceConfigMap, s.namespace, err)
	}

	// configmap.data:
	//   2456: demo-ns/demo-service:8893
	//   7676: demo-ns2/demo-service2:8893
	//   5556: demo-ns2/demo-service2:7791

	valuePrefix := fmt.Sprintf("%s/%s", s.namespace, s.name)
	for k, v := range tcpcm.Data {
		split := strings.Split(v, ":")
		if len(split) != 2 {
			continue
		}
		if !strings.EqualFold(split[0], valuePrefix) {
			continue
		}
		servicePort, err := strconv.Atoi(split[1])
		if err != nil {
			continue
		}
		protocol := TCP
		externalPort, err := strconv.Atoi(k)
		if err != nil {
			continue
		}
		tcpResultMap[servicePort] = &ExternalAccessInfo{ServicePort: servicePort, Protocol: protocol, ExternalPort: &externalPort}
	}
	for k, v := range udpcm.Data {
		split := strings.Split(v, ":")
		if len(split) != 2 {
			continue
		}
		if !strings.EqualFold(split[0], valuePrefix) {
			continue
		}
		servicePort, err := strconv.Atoi(split[1])
		if err != nil {
			continue
		}
		protocol := UDP
		externalPort, err := strconv.Atoi(k)
		if err != nil {
			continue
		}
		udpResultMap[servicePort] = &ExternalAccessInfo{ServicePort: servicePort, Protocol: protocol, ExternalPort: &externalPort}
	}
	return tcpResultMap, udpResultMap, nil
}

// getExternalAccess get external info
func (s *ExternalAccess) getExternalAccess() ([]ExternalAccessInfo, *errcode.ErrorInfo) {
	tcpResultMap, udpResultMap, err := s.GetExternalAccessConfigMap()
	if err != nil {
		return nil, errcode.BadRequest(err)
	}
	// not in configmap but in service.spec
	var service v1.Service
	err = s.client.Get(s.ctx, types.NamespacedName{Namespace: s.namespace, Name: s.name}, &service)
	if err != nil {
		return nil, errcode.BadRequest(err)
	}
	for _, items := range service.Spec.Ports {
		if items.Protocol == TCP {
			if _, ok := tcpResultMap[int(items.Port)]; !ok {
				tcpResultMap[int(items.Port)] = &ExternalAccessInfo{ServicePort: int(items.Port), Protocol: TCP, ServicePortName: items.Name}
			} else {
				info := tcpResultMap[int(items.Port)]
				info.ServicePortName = items.Name
			}
		} else if items.Protocol == UDP {
			if _, ok := udpResultMap[int(items.Port)]; !ok {
				udpResultMap[int(items.Port)] = &ExternalAccessInfo{ServicePort: int(items.Port), Protocol: UDP, ServicePortName: items.Name}
			} else {
				info := udpResultMap[int(items.Port)]
				info.ServicePortName = items.Name
			}
		}
	}
	// change map to list
	var result []ExternalAccessInfo
	for _, v := range tcpResultMap {
		result = append(result, *v)
	}
	for _, v := range udpResultMap {
		result = append(result, *v)
	}

	return result, nil
}

// DeleteExternalAccess delete external service
func (s *ExternalAccess) DeleteExternalAccess() error {
	// get configmap
	var tcpcm v1.ConfigMap
	var udpcm v1.ConfigMap
	err := s.client.Get(s.ctx, types.NamespacedName{Namespace: s.NginxNamespace, Name: s.NginxTcpServiceConfigMap}, &tcpcm)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("can not find configmap %s in %s from cluster, %v", s.NginxTcpServiceConfigMap, s.NginxNamespace, err)
	}
	err = s.client.Get(s.ctx, types.NamespacedName{Namespace: s.NginxNamespace, Name: s.NginxUdpServiceConfigMap}, &udpcm)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("can not find configmap %s in %s from cluster, %v", s.NginxUdpServiceConfigMap, s.NginxNamespace, err)
	}
	// clear old
	value := fmt.Sprintf("%s/%s:", s.namespace, s.name)
	for k, v := range tcpcm.Data {
		if strings.HasPrefix(v, value) {
			delete(tcpcm.Data, k)
		}
	}
	for k, v := range udpcm.Data {
		if strings.HasPrefix(v, value) {
			delete(udpcm.Data, k)
		}
	}
	err = s.client.Update(s.ctx, &tcpcm)
	if err != nil {
		return fmt.Errorf("update fail")
	}
	err = s.client.Update(s.ctx, &udpcm)
	if err != nil {
		return fmt.Errorf("update fail")
	}
	return nil
}
