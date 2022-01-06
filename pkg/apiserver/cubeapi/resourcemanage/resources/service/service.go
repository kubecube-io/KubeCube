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

	"github.com/kubecube-io/kubecube/pkg/clog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources"
	mgrclient "github.com/kubecube-io/kubecube/pkg/multicluster/client"
)

type Service struct {
	ctx       context.Context
	client    mgrclient.Client
	namespace string
	filter    resources.Filter
}

func NewService(client mgrclient.Client, namespace string, filter resources.Filter) Service {
	ctx := context.Background()
	return Service{
		ctx:       ctx,
		client:    client,
		namespace: namespace,
		filter:    filter,
	}
}

func (s *Service) GetExtendServices() resources.K8sJson {
	resultMap := make(resources.K8sJson)
	// get service list from k8s cluster
	var serviceList corev1.ServiceList
	err := s.client.Cache().List(s.ctx, &serviceList, client.InNamespace(s.namespace))
	if err != nil {
		clog.Error("can not find service from cluster, %v", err)
		return nil
	}
	resultMap["total"] = len(serviceList.Items)

	// filter list by selector/sort/page
	serviceListJson, err := json.Marshal(serviceList)
	if err != nil {
		clog.Error("convert serviceList to json fail, %v", err)
		return nil
	}
	serviceListJson = s.filter.FilterResult(serviceListJson)
	err = json.Unmarshal(serviceListJson, &serviceList)
	if err != nil {
		clog.Error("convert json to serviceList fail, %v", err)
		return nil
	}

	// add pod status info
	resultList := s.addExtendInfo(serviceList)

	resultMap["items"] = resultList

	return resultMap
}

// get external ips
func (s *Service) addExtendInfo(serviceList corev1.ServiceList) resources.K8sJsonArr {
	resultList := make(resources.K8sJsonArr, 0)

	for _, service := range serviceList.Items {
		ips := make([]string, 0)

		switch service.Spec.Type {
		case corev1.ServiceTypeNodePort:
			// NodePort: get nodeName from ep, and get ip from node
			var endpoints corev1.Endpoints
			err := s.client.Cache().Get(s.ctx, types.NamespacedName{Namespace: service.Namespace, Name: service.Name}, &endpoints)
			if err != nil {
				clog.Error("can not find endpoints from cluster, %v", err)
				return nil
			}
			nodeNameMap := make(map[string]struct{})
			for _, subnet := range endpoints.Subsets {
				for _, address := range subnet.Addresses {
					var node corev1.Node
					err = s.client.Cache().Get(s.ctx, types.NamespacedName{Name: *address.NodeName}, &node)
					if err != nil {
						clog.Error("can not find node from cluster, %v", err)
						return nil
					}
					if _, ok := nodeNameMap[node.Name]; ok {
						continue
					}

					nodeIp := ""
					for _, nodeAddress := range node.Status.Addresses {
						if nodeAddress.Type == corev1.NodeExternalIP {
							nodeIp = nodeAddress.Address
							ips = append(ips, nodeIp)
						}
					}
					if nodeIp == "" {
						for _, nodeAddress := range node.Status.Addresses {
							if nodeAddress.Type == corev1.NodeInternalIP {
								nodeIp = nodeAddress.Address
								ips = append(ips, nodeIp)
							}
						}
					}
					nodeNameMap[node.Name] = struct{}{}
				}
			}
		case corev1.ServiceTypeLoadBalancer:
			// LoadBalancerr: get ip from status.LoadBalancer.Ingress
			if service.Status.LoadBalancer.Ingress != nil {
				for _, ingress := range service.Status.LoadBalancer.Ingress {
					if ingress.IP != "" {
						ips = append(ips, ingress.IP)
					}
				}
			}
		case corev1.ServiceTypeExternalName:
			ips = append(ips, service.Spec.ClusterIP)
		}

		// create result map
		result := make(resources.K8sJson)
		result["metadata"] = service.ObjectMeta
		result["spec"] = service.Spec
		result["status"] = service.Status
		result["externalIps"] = ips
		resultList = append(resultList, result)
	}
	return resultList
}
