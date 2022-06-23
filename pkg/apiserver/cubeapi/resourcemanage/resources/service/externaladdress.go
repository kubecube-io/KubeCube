package service

import (
	"errors"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcemanage "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/handle"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/enum"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
)

func init() {
	resourcemanage.SetExtendHandler(enum.ExternalAccessAddressResourceType, AddressHandle)
}

func AddressHandle(param resourcemanage.ExtendParams) (interface{}, error) {
	access := resources.NewSimpleAccess(param.Cluster, param.Username, param.Namespace)
	if allow := access.AccessAllow("", "services", "list"); !allow {
		return nil, errors.New(errcode.ForbiddenErr.Message)
	}
	kubernetes := clients.Interface().Kubernetes(param.Cluster)
	if kubernetes == nil {
		return nil, errors.New(errcode.ClusterNotFoundError(param.Cluster).Message)
	}
	externalAccess := NewExternalAccess(kubernetes.Direct(), param.Namespace, param.ResourceName, param.Filter, param.NginxNamespace, param.NginxTcpServiceConfigMap, param.NginxUdpServiceConfigMap)
	return externalAccess.GetExternalIP()
}

// GetExternalIP ingress-nginx pod status host IPs
func (s *ExternalAccess) GetExternalIP() ([]string, error) {
	var podList v1.PodList
	nginxLabel := map[string]string{
		"app.kubernetes.io/component": "controller",
		"app.kubernetes.io/name":      "ingress-nginx",
	}

	err := s.client.List(s.ctx, &podList, &client.ListOptions{Namespace: s.NginxNamespace, LabelSelector: labels.SelectorFromSet(nginxLabel)})
	if err != nil {
		clog.Error("can not find pod ingress-nginx in %s from cluster, %v", s.NginxNamespace, err)
		return nil, err
	}

	var hostIps []string
	for _, pod := range podList.Items {
		if pod.Status.HostIP != "" {
			hostIps = append(hostIps, pod.Status.HostIP)
		}
	}

	return hostIps, nil
}
