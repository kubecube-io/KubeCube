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
	"fmt"
	"strconv"
	"strings"

	"context"

	"github.com/kubecube-io/kubecube/pkg/clog"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	jsoniter "github.com/json-iterator/go"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources"
	mgrclient "github.com/kubecube-io/kubecube/pkg/multicluster/client"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

const (
	INGRESS_NS   = "ingress-nginx"
	TCP_SERVICES = "tcp-services"
	UDP_SERVICES = "udp-services"
	TCP          = "TCP"
	UDP          = "UDP"
)

type ExternalAccess struct {
	ctx       context.Context
	client    mgrclient.Client
	namespace string
	name      string
	filter    resources.Filter
}

func NewExternalAccess(client mgrclient.Client, namespace string, name string, filter resources.Filter) ExternalAccess {
	ctx := context.Background()
	return ExternalAccess{
		ctx:       ctx,
		client:    client,
		namespace: namespace,
		name:      name,
		filter:    filter,
	}
}

type ExternalAccessInfo struct {
	ServicePort   int    `json:"servicePort,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
	ExternalPorts []int  `json:"externalPorts,omitempty"`
}

// ingress-nginx pod status host IPs
func (s *ExternalAccess) GetExternalIP() []string {
	var podList v1.PodList
	nginxLable := map[string]string{
		"app.kubernetes.io/component": "controller",
		"app.kubernetes.io/name":      "ingress-nginx",
	}

	err := s.client.Cache().List(s.ctx, &podList, &client.ListOptions{Namespace: INGRESS_NS, LabelSelector: labels.SelectorFromSet(nginxLable)})
	if err != nil {
		clog.Error("can not find pod ingress-nginx in %s from cluster, %v", INGRESS_NS, err)
		return nil
	}

	var hostIps []string
	for _, pod := range podList.Items {
		if pod.Status.HostIP != "" {
			hostIps = append(hostIps, pod.Status.HostIP)
		}
	}

	return hostIps
}

func (s *ExternalAccess) SetExternalAccess(body []byte) error {
	var externalServices []ExternalAccessInfo
	err := json.Unmarshal(body, &externalServices)
	if err != nil {
		return fmt.Errorf("can not parse body info")
	}

	// get service
	var service v1.Service
	err = s.client.Cache().Get(s.ctx, types.NamespacedName{Namespace: s.namespace, Name: s.name}, &service)
	if err != nil {
		return err
	}

	// get configmap
	var tcpcm v1.ConfigMap
	var udpcm v1.ConfigMap
	err = s.client.Cache().Get(s.ctx, types.NamespacedName{Namespace: INGRESS_NS, Name: TCP_SERVICES}, &tcpcm)
	if err != nil {
		return fmt.Errorf("can not find configmap tcp-services in kube-system from cluster, %v", err)
	}
	err = s.client.Cache().Get(s.ctx, types.NamespacedName{Namespace: INGRESS_NS, Name: UDP_SERVICES}, &udpcm)
	if err != nil {
		return fmt.Errorf("can not find configmap udp-services in kube-system from cluster, %v", err)
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
		for _, ep := range es.ExternalPorts {
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
	}
	err = s.client.Direct().Update(s.ctx, &tcpcm)
	if err != nil {
		return fmt.Errorf("update fail")
	}
	err = s.client.Direct().Update(s.ctx, &udpcm)
	if err != nil {
		return fmt.Errorf("update fail")
	}
	return nil
}

// get external info
func (s *ExternalAccess) GetExternalAccess() ([]ExternalAccessInfo, error) {
	var tcpcm v1.ConfigMap
	var udpcm v1.ConfigMap
	err := s.client.Cache().Get(s.ctx, types.NamespacedName{Namespace: INGRESS_NS, Name: "tcp-services"}, &tcpcm)
	if err != nil {
		return nil, fmt.Errorf("can not find configmap tcp-services in kube-system from cluster, %v", err)
	}
	err = s.client.Cache().Get(s.ctx, types.NamespacedName{Namespace: INGRESS_NS, Name: "udp-services"}, &udpcm)
	if err != nil {
		return nil, fmt.Errorf("can not find configmap udp-services in kube-system from cluster, %v", err)
	}

	// configmap.data:
	//   2456: demo-ns/demo-service:8893
	//   7676: demo-ns2/demo-service2:8893
	//   5556: demo-ns2/demo-service2:7791
	tcpResultMap := make(map[int]ExternalAccessInfo)
	udpResultMap := make(map[int]ExternalAccessInfo)
	valuePrefix := fmt.Sprintf("%s/%s:", s.namespace, s.name)
	for k, v := range tcpcm.Data {
		if !strings.HasPrefix(v, valuePrefix) {
			continue
		}
		servicePort, err := strconv.Atoi(v[len(valuePrefix):])
		if err != nil {
			continue
		}
		protocol := TCP
		externalPort, err := strconv.Atoi(k)
		if err != nil {
			continue
		}
		if externalAccessInfo, ok := tcpResultMap[servicePort]; ok {
			externalAccessInfo.ExternalPorts = append(externalAccessInfo.ExternalPorts, externalPort)
			tcpResultMap[servicePort] = externalAccessInfo
		} else {
			tcpResultMap[servicePort] = ExternalAccessInfo{servicePort, protocol, []int{externalPort}}
		}

	}
	for k, v := range udpcm.Data {
		if !strings.HasPrefix(v, valuePrefix) {
			continue
		}
		servicePort, err := strconv.Atoi(v[len(valuePrefix):])
		if err != nil {
			continue
		}
		protocol := UDP
		externalPort, err := strconv.Atoi(k)
		if err != nil {
			continue
		}
		if externalAccessInfo, ok := udpResultMap[servicePort]; ok {
			externalAccessInfo.ExternalPorts = append(externalAccessInfo.ExternalPorts, externalPort)
			udpResultMap[servicePort] = externalAccessInfo
		} else {
			udpResultMap[servicePort] = ExternalAccessInfo{servicePort, protocol, []int{externalPort}}
		}
	}
	// not in configmap but in service.spec
	var service v1.Service
	err = s.client.Cache().Get(s.ctx, types.NamespacedName{Namespace: s.namespace, Name: s.name}, &service)
	if err != nil {
		return nil, err
	}
	for _, items := range service.Spec.Ports {
		if items.Protocol == TCP {
			if _, ok := tcpResultMap[int(items.Port)]; !ok {
				tcpResultMap[int(items.Port)] = ExternalAccessInfo{int(items.Port), TCP, []int{}}
			}
		} else if items.Protocol == UDP {
			if _, ok := udpResultMap[int(items.Port)]; !ok {
				udpResultMap[int(items.Port)] = ExternalAccessInfo{int(items.Port), UDP, []int{}}
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

// delete external service
func (s *ExternalAccess) DeleteExternalAccess() error {
	// get configmap
	var tcpcm v1.ConfigMap
	var udpcm v1.ConfigMap
	err := s.client.Cache().Get(s.ctx, types.NamespacedName{Namespace: INGRESS_NS, Name: "tcp-services"}, &tcpcm)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("can not find configmap tcp-services in kube-system from cluster, %v", err)
	}
	err = s.client.Cache().Get(s.ctx, types.NamespacedName{Namespace: INGRESS_NS, Name: "udp-services"}, &udpcm)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("can not find configmap udp-services in kube-system from cluster, %v", err)
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
	err = s.client.Direct().Update(s.ctx, &tcpcm)
	if err != nil {
		return fmt.Errorf("update fail")
	}
	err = s.client.Direct().Update(s.ctx, &udpcm)
	if err != nil {
		return fmt.Errorf("update fail")
	}
	return nil
}
