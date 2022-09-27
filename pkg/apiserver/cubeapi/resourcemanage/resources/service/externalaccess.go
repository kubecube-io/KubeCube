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
	"errors"
	"fmt"
	"k8s.io/apimachinery/pkg/util/intstr"
	"net/http"
	"strconv"
	"strings"

	jsoniter "github.com/json-iterator/go"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
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
	filter                   filter.Filter
	NginxNamespace           string
	NginxTcpServiceConfigMap string
	NginxUdpServiceConfigMap string
}

func NewExternalAccess(client client.Client, namespace string, name string, filter filter.Filter, nginxNs string, tcpCm string, udpCm string) ExternalAccess {
	ctx := context.Background()
	return ExternalAccess{
		ctx:                      ctx,
		client:                   client,
		namespace:                namespace,
		name:                     name,
		filter:                   filter,
		NginxNamespace:           nginxNs,
		NginxTcpServiceConfigMap: tcpCm,
		NginxUdpServiceConfigMap: udpCm,
	}
}

type ExternalAccessInfo struct {
	ServicePort  int    `json:"servicePort,omitempty"`
	Protocol     string `json:"protocol,omitempty"`
	ExternalPort *int   `json:"externalPort,omitempty"`
}

func init() {
	resourcemanage.SetExtendHandler(enum.ExternalAccessResourceType, ExternalHandle)
}

func ExternalHandle(param resourcemanage.ExtendParams) (interface{}, error) {
	access := resources.NewSimpleAccess(param.Cluster, param.Username, param.Namespace)
	kubernetes := clients.Interface().Kubernetes(param.Cluster)
	if kubernetes == nil {
		return nil, errors.New(errcode.ClusterNotFoundError(param.Cluster).Message)
	}
	externalAccess := NewExternalAccess(kubernetes.Direct(), param.Namespace, param.ResourceName, param.Filter, param.NginxNamespace, param.NginxTcpServiceConfigMap, param.NginxUdpServiceConfigMap)
	switch param.Action {
	case http.MethodGet:
		if allow := access.AccessAllow("", "services", "list"); !allow {
			return nil, errors.New(errcode.ForbiddenErr.Message)
		}
		return externalAccess.GetExternalAccess()
	case http.MethodPost:
		if allow := access.AccessAllow("", "services", "create"); !allow {
			return nil, errors.New(errcode.ForbiddenErr.Message)
		}
		var externalServices []ExternalAccessInfo
		err := json.Unmarshal(param.Body, &externalServices)
		if err != nil {
			return nil, errors.New(errcode.InvalidBodyFormat.Message)
		}
		err = externalAccess.SetExternalAccess(externalServices)
		if err != nil {
			return nil, err
		}
		return "success", nil
	default:
		return nil, errors.New(errcode.InvalidHttpMethod.Message)
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
	testService.Spec.Ports = ports
	err = s.client.Create(s.ctx, testService, client.DryRunAll)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
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

// GetExternalAccess get external info
func (s *ExternalAccess) GetExternalAccess() ([]ExternalAccessInfo, error) {
	var tcpcm v1.ConfigMap
	var udpcm v1.ConfigMap
	err := s.client.Get(s.ctx, types.NamespacedName{Namespace: s.NginxNamespace, Name: s.NginxTcpServiceConfigMap}, &tcpcm)
	if err != nil {
		return nil, fmt.Errorf("can not find configmap %s in %s from cluster, %v", s.NginxTcpServiceConfigMap, s.NginxNamespace, err)
	}
	err = s.client.Get(s.ctx, types.NamespacedName{Namespace: s.NginxNamespace, Name: s.NginxUdpServiceConfigMap}, &udpcm)
	if err != nil {
		return nil, fmt.Errorf("can not find configmap %s in %s from cluster, %v", s.NginxUdpServiceConfigMap, s.namespace, err)
	}

	// configmap.data:
	//   2456: demo-ns/demo-service:8893
	//   7676: demo-ns2/demo-service2:8893
	//   5556: demo-ns2/demo-service2:7791
	tcpResultMap := make(map[int]ExternalAccessInfo)
	udpResultMap := make(map[int]ExternalAccessInfo)
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
		tcpResultMap[servicePort] = ExternalAccessInfo{servicePort, protocol, &externalPort}
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
		protocol := UDP
		externalPort, err := strconv.Atoi(k)
		if err != nil {
			continue
		}
		udpResultMap[servicePort] = ExternalAccessInfo{servicePort, protocol, &externalPort}
	}
	// not in configmap but in service.spec
	var service v1.Service
	err = s.client.Get(s.ctx, types.NamespacedName{Namespace: s.namespace, Name: s.name}, &service)
	if err != nil {
		return nil, err
	}
	for _, items := range service.Spec.Ports {
		if items.Protocol == TCP {
			if _, ok := tcpResultMap[int(items.Port)]; !ok {
				tcpResultMap[int(items.Port)] = ExternalAccessInfo{int(items.Port), TCP, nil}
			}
		} else if items.Protocol == UDP {
			if _, ok := udpResultMap[int(items.Port)]; !ok {
				udpResultMap[int(items.Port)] = ExternalAccessInfo{int(items.Port), UDP, nil}
			}
		}
	}
	// change map to list
	var result []ExternalAccessInfo
	for _, v := range tcpResultMap {
		result = append(result, v)
	}
	for _, v := range udpResultMap {
		result = append(result, v)
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
