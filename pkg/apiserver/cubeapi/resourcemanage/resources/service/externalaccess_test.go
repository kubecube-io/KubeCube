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

package service_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kubecube-io/kubecube/pkg/apis"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/service"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	mgrclient "github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"github.com/kubecube-io/kubecube/pkg/multicluster/client/fake"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
)

var _ = Describe("Externalaccess", func() {
	var (
		externalAccess service.ExternalAccess
		cli            mgrclient.Client
		ns             = "namespace-test"
		serviceName    = "service-test"
		serviceName2   = "service-test2"
		pod1           corev1.Pod
		pod2           corev1.Pod
		podList        corev1.PodList
		hostIp1        = "192.168.0.1"
		hostIp2        = "192.168.0.2"
		service1       corev1.Service
		service2       corev1.Service
		tcpcm          corev1.ConfigMap
		udpcm          corev1.ConfigMap
	)
	BeforeEach(func() {
		nginxLable := map[string]string{
			"app.kubernetes.io/component": "controller",
			"app.kubernetes.io/name":      "ingress-nginx",
		}
		pod1 = corev1.Pod{
			TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: service.INGRESS_NS, Labels: nginxLable},
			Status:     corev1.PodStatus{HostIP: hostIp1},
		}
		pod2 = corev1.Pod{
			TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{Name: "pod2", Namespace: service.INGRESS_NS, Labels: nginxLable},
			Status:     corev1.PodStatus{HostIP: hostIp2},
		}
		podList = corev1.PodList{
			Items: []corev1.Pod{pod1, pod2},
		}
		service1 = corev1.Service{
			TypeMeta:   metav1.TypeMeta{Kind: "Service", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{Name: serviceName, Namespace: ns},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{Protocol: service.TCP, Port: 7777}, {Protocol: service.UDP, Port: 8888}, {Protocol: service.TCP, Port: 9999}},
			},
		}
		service2 = corev1.Service{
			TypeMeta:   metav1.TypeMeta{Kind: "Service", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{Name: serviceName2, Namespace: ns},
		}
		tcpcm = corev1.ConfigMap{
			TypeMeta:   metav1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{Name: service.TCP_SERVICES, Namespace: service.INGRESS_NS},
			Data:       map[string]string{"5000": "namespace-test/service-test2:5555", "5500": "namespace-test/service-test2:5555", "5550": "ns/serv:5551"},
		}
		udpcm = corev1.ConfigMap{
			TypeMeta:   metav1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{Name: service.UDP_SERVICES, Namespace: service.INGRESS_NS},
			Data:       map[string]string{"6000": "namespace-test/service-test2:6666", "6600": "namespace-test/service-test2:6666", "6660": "ns/serv:6661"},
		}
	})
	JustBeforeEach(func() {
		scheme := runtime.NewScheme()
		apis.AddToScheme(scheme)
		corev1.AddToScheme(scheme)
		appsv1.AddToScheme(scheme)
		opts := &fake.Options{
			Scheme:               scheme,
			Objs:                 []client.Object{&service1, &service2, &tcpcm, &udpcm},
			ClientSetRuntimeObjs: []runtime.Object{},
			Lists:                []client.ObjectList{&podList},
		}
		multicluster.InitFakeMultiClusterMgrWithOpts(opts)
		clients.InitCubeClientSetWithOpts(nil)
		cli = clients.Interface().Kubernetes(constants.LocalCluster)
		Expect(cli).NotTo(BeNil())
		externalAccess = service.NewExternalAccess(cli, ns, serviceName, resources.Filter{Limit: 10})
	})

	It("test get external address", func() {
		ret := externalAccess.GetExternalIP()
		Expect(len(ret)).To(Equal(2))
		Expect(ret[0]).To(Equal(hostIp1))
		Expect(ret[1]).To(Equal(hostIp2))
	})

	It("test set external access and get external and delete external", func() {
		externalServices := make([]service.ExternalAccessInfo, 2)
		externalServices[0] = service.ExternalAccessInfo{
			ServicePort:   7777,
			Protocol:      service.TCP,
			ExternalPorts: []int{7000, 7700, 7770},
		}
		externalServices[1] = service.ExternalAccessInfo{
			ServicePort:   8888,
			Protocol:      service.UDP,
			ExternalPorts: []int{8000, 8800, 8880},
		}
		body, err := json.Marshal(externalServices)
		Expect(err).To(BeNil())
		err = externalAccess.SetExternalAccess(body)
		Expect(err).To(BeNil())
		ret, err := externalAccess.GetExternalAccess()
		Expect(err).To(BeNil())
		for _, item := range ret {
			switch item.ServicePort {
			case 7777:
				Expect(item.Protocol).To(Equal(service.TCP))
				Expect(item.ExternalPorts).To(ContainElements(7000, 7700, 7770))
			case 8888:
				Expect(item.Protocol).To(Equal(service.UDP))
				Expect(item.ExternalPorts).To(ContainElements(8000, 8800, 8880))
			case 9999:
				// not exist in tcpcm or udpcm, but exist in service ports
				Expect(item.Protocol).To(Equal(service.TCP))
				Expect(item.ServicePort).To(Equal(9999))
			default:
				Panic()
			}
		}
		err = externalAccess.DeleteExternalAccess()
		Expect(err).To(BeNil())
		ret, err = externalAccess.GetExternalAccess()
		Expect(err).To(BeNil())
		for _, item := range ret {
			switch item.ServicePort {
			case 7777:
				Expect(item.Protocol).To(Equal(service.TCP))
				Expect(len(item.ExternalPorts)).To(Equal(0))
			case 8888:
				Expect(item.Protocol).To(Equal(service.UDP))
				Expect(len(item.ExternalPorts)).To(Equal(0))
			case 9999:
				Expect(item.Protocol).To(Equal(service.TCP))
				Expect(len(item.ExternalPorts)).To(Equal(0))
			default:
				panic("no match result")
			}
		}
	})

	It("test get external access", func() {
		ea := service.NewExternalAccess(cli, ns, serviceName2, resources.Filter{Limit: 10})
		ret, err := ea.GetExternalAccess()
		Expect(err).To(BeNil())
		for _, item := range ret {
			switch item.ServicePort {
			case 5555:
				Expect(item.Protocol).To(Equal(service.TCP))
				Expect(item.ExternalPorts).To(ContainElements(5000, 5500))
			case 6666:
				Expect(item.Protocol).To(Equal(service.UDP))
				Expect(item.ExternalPorts).To(ContainElements(6000, 6600))
			default:
				panic("no match result")
			}
		}
	})

	It("test set external access, service port no exist", func() {
		externalServices := make([]service.ExternalAccessInfo, 2)
		externalServices[1] = service.ExternalAccessInfo{ // not exist in this service
			ServicePort:   5555,
			Protocol:      service.UDP,
			ExternalPorts: []int{5000},
		}
		body, err := json.Marshal(externalServices)
		Expect(err).To(BeNil())
		err = externalAccess.SetExternalAccess(body)
		Expect(err.Error()).To(Equal("the service port is not exist"))
	})
})
