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

package cluster

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
	v1 "github.com/kubecube-io/kubecube/pkg/apis/quota/v1"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	mgrclient "github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"github.com/kubecube-io/kubecube/pkg/quota"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/strproc"
)

// makeClusterInfos make cluster info with clusters given
// todo split metric and cluster info into two apis
func makeClusterInfos(ctx context.Context, clusters clusterv1.ClusterList, pivotCli mgrclient.Client, statusFilter string) ([]clusterInfo, error) {
	// populate cluster info one by one
	infos := make([]clusterInfo, 0)
	for _, item := range clusters.Items {
		v := item.Name
		info := clusterInfo{ClusterName: v}

		cluster := clusterv1.Cluster{}
		clusterKey := types.NamespacedName{Name: v}
		err := pivotCli.Direct().Get(ctx, clusterKey, &cluster)
		if err != nil {
			clog.Warn("get cluster %v failed: %v", v, err)
			continue
		}

		state := cluster.Status.State
		if state == nil {
			processState := clusterv1.ClusterProcessing
			state = &processState
		}

		info.Status = string(*state)
		info.ClusterDescription = cluster.Spec.Description
		info.CreateTime = cluster.CreationTimestamp.Time
		info.IsMemberCluster = cluster.Spec.IsMemberCluster
		info.HarborAddr = cluster.Spec.HarborAddr
		info.KubeApiServer = cluster.Spec.KubernetesAPIEndpoint
		info.NetworkType = cluster.Spec.NetworkType
		info.IngressDomainSuffix = cluster.Spec.IngressDomainSuffix

		internalCluster, err := multicluster.Interface().Get(v)
		if internalCluster != nil && err != nil {
			info.Status = string(clusterv1.ClusterAbnormal)
		}
		if internalCluster == nil {
			if len(statusFilter) == 0 {
				infos = append(infos, info)
			} else if info.Status == statusFilter {
				infos = append(infos, info)
			}
			continue
		}

		cli := internalCluster.Client

		// todo(weilaaa): context may be exceed if metrics query timeout
		// will deprecated in v2.0.x
		nodesMc, err := cli.Metrics().MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
		if err != nil {
			// record error from metric server, but ensure return normal
			clog.Warn("get cluster %v nodes metrics failed: %v", v, err)
		} else {
			for _, m := range nodesMc.Items {
				info.UsedCPU += int(m.Usage.Cpu().MilliValue())                                         // 1000 m
				info.UsedMem += int(m.Usage.Memory().ScaledValue(resource.Mega))                        // 1024 Mi
				info.UsedStorage += int(m.Usage.Storage().ScaledValue(resource.Mega))                   // 1024 Mi
				info.UsedStorageEphemeral += int(m.Usage.StorageEphemeral().ScaledValue(resource.Mega)) // 1024 Mi
			}
		}

		nodes := corev1.NodeList{}
		err = cli.Cache().List(ctx, &nodes)
		if err != nil {
			return nil, fmt.Errorf("get cluster %v nodes failed: %v", v, err)
		}

		info.NodeCount = len(nodes.Items)

		for _, n := range nodes.Items {
			info.TotalCPU += int(n.Status.Capacity.Cpu().MilliValue())                                         // 1000 m
			info.TotalMem += int(n.Status.Capacity.Memory().ScaledValue(resource.Mega))                        // 1024 Mi
			info.TotalStorage += int(n.Status.Capacity.Storage().ScaledValue(resource.Mega))                   // 1024 Mi
			info.TotalStorageEphemeral += int(n.Status.Capacity.StorageEphemeral().ScaledValue(resource.Mega)) // 1024 Mi
		}

		ns := corev1.NamespaceList{}
		err = cli.Cache().List(ctx, &ns)
		if err != nil {
			return nil, fmt.Errorf("get cluster %v namespace failed: %v", v, err)
		}

		info.NamespaceCount = len(ns.Items)

		clusterNonTerminatedPodsList, err := getPodsInCluster(cli)
		if err != nil {
			return nil, err
		}

		for _, pod := range clusterNonTerminatedPodsList.Items {
			req, limit := podRequestsAndLimits(&pod)
			cpuReq, cpuLimit, memoryReq, memoryLimit := req[corev1.ResourceCPU], limit[corev1.ResourceCPU], req[corev1.ResourceMemory], limit[corev1.ResourceMemory]
			info.UsedCPURequest += int(cpuReq.MilliValue())                  // 1000 m
			info.UsedCPULimit += int(cpuLimit.MilliValue())                  // 1000 m
			info.UsedMemRequest += int(memoryReq.ScaledValue(resource.Mega)) // 1024 Mi
			info.UsedMemLimit += int(memoryLimit.ScaledValue(resource.Mega)) // 1024 Mi
		}

		if len(statusFilter) == 0 {
			infos = append(infos, info)
		} else if info.Status == statusFilter {
			infos = append(infos, info)
		}
	}

	return infos, nil
}

func podRequestsAndLimits(pod *corev1.Pod) (reqs, limits corev1.ResourceList) {
	reqs, limits = corev1.ResourceList{}, corev1.ResourceList{}
	for _, container := range pod.Spec.Containers {
		addResourceList(reqs, container.Resources.Requests)
		addResourceList(limits, container.Resources.Limits)
	}
	// init containers define the minimum of any resource
	for _, container := range pod.Spec.InitContainers {
		maxResourceList(reqs, container.Resources.Requests)
		maxResourceList(limits, container.Resources.Limits)
	}

	// Add overhead for running a pod to the sum of requests and to non-zero limits:
	if pod.Spec.Overhead != nil {
		addResourceList(reqs, pod.Spec.Overhead)

		for name, quantity := range pod.Spec.Overhead {
			if value, ok := limits[name]; ok && !value.IsZero() {
				value.Add(quantity)
				limits[name] = value
			}
		}
	}
	return
}

// addResourceList adds the resources in newList to list
func addResourceList(list, new corev1.ResourceList) {
	for name, quantity := range new {
		if value, ok := list[name]; !ok {
			list[name] = quantity.DeepCopy()
		} else {
			value.Add(quantity)
			list[name] = value
		}
	}
}

// maxResourceList sets list to the greater of list/newList for every resource
// either list
func maxResourceList(list, new corev1.ResourceList) {
	for name, quantity := range new {
		if value, ok := list[name]; !ok {
			list[name] = quantity.DeepCopy()
			continue
		} else {
			if quantity.Cmp(value) > 0 {
				list[name] = quantity.DeepCopy()
			}
		}
	}
}

func getPodsInCluster(cli mgrclient.Client) (*corev1.PodList, error) {
	fieldSelector, err := fields.ParseSelector("status.phase!=" + string(corev1.PodSucceeded) + ",status.phase!=" + string(corev1.PodFailed))
	if err != nil {
		return nil, err
	}
	podList := &corev1.PodList{}
	// todo: use cache as soon as cache support for complicated field selector
	err = cli.Direct().List(context.TODO(), podList, &client.ListOptions{FieldSelector: fieldSelector})
	if err != nil {
		return nil, err
	}
	return podList, nil
}

func makeMonitorInfo(ctx context.Context, cluster string) (*monitorInfo, error) {
	cli := clients.Interface().Kubernetes(cluster)
	if cli == nil {
		return nil, fmt.Errorf("cluster %v abnormal", cluster)
	}

	nodesMC, err := cli.Metrics().MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("get cluster %v nodes metrics failed: %v", cluster, err)
	}

	info := &monitorInfo{}
	for _, m := range nodesMC.Items {
		info.UsedCPU += strproc.Str2int(m.Usage.Cpu().String())/1000000 + 1
		info.UsedMem += strproc.Str2int(m.Usage.Memory().String()) / 1024
		info.UsedStorage += (strproc.Str2int(m.Usage.Storage().String()) + 1) / 1024
		info.UsedStorageEphemeral += (strproc.Str2int(m.Usage.StorageEphemeral().String()) + 1) / 1024
	}

	nodes := corev1.NodeList{}
	err = cli.Cache().List(ctx, &nodes)
	if err != nil {
		return nil, fmt.Errorf("get cluster %v nodes failed: %v", cluster, err)
	}

	info.NodeCount = len(nodes.Items)

	for _, n := range nodes.Items {
		info.TotalCPU += strproc.Str2int(n.Status.Capacity.Cpu().String()) * 1000
		info.TotalMem += strproc.Str2int(n.Status.Capacity.Memory().String()) / 1024
		info.TotalStorage += (strproc.Str2int(n.Status.Capacity.Storage().String()) + 1) / 1024
		info.TotalStorageEphemeral += (strproc.Str2int(n.Status.Capacity.StorageEphemeral().String()) + 1) / 1024
	}

	ns := corev1.NamespaceList{}
	err = cli.Cache().List(ctx, &ns)
	if err != nil {
		return nil, fmt.Errorf("get cluster %v namespace failed: %v", cluster, err)
	}

	info.NamespaceCount = len(ns.Items)

	return info, nil
}

// isRelateWith return true if third level namespace exist under of ancestor namespace
func isRelateWith(namespace string, cli cache.Cache, depth string, ctx context.Context) (bool, error) {
	if depth == constants.HncCurrentDepth {
		return true, nil
	}

	hncLabel := namespace + constants.HncSuffix
	nsList := corev1.NamespaceList{}

	err := cli.List(ctx, &nsList)
	if err != nil {
		return false, err
	}

	for _, ns := range nsList.Items {
		if d, ok := ns.Labels[hncLabel]; ok {
			if d == depth {
				return true, nil
			}
		}
	}

	return false, nil
}

// listClusterNames list all clusters name
func listClusterNames() []string {
	clusterNames := make([]string, 0)
	clusters := multicluster.Interface().FuzzyCopy()

	for _, c := range clusters {
		clusterNames = append(clusterNames, c.Name)
	}

	return clusterNames
}

// getClustersByNamespace get clusters where the namespace work in
func getClustersByNamespace(namespace string, ctx context.Context) ([]string, error) {
	clusterNames := make([]string, 0)
	clusters := multicluster.Interface().FuzzyCopy()
	key := types.NamespacedName{Name: namespace}

	for _, cluster := range clusters {
		cli := cluster.Client.Cache()
		ns := corev1.Namespace{}
		isRelated := true

		err := cli.Get(ctx, key, &ns)
		if err != nil {
			if errors.IsNotFound(err) {
				clog.Debug("cluster %s not work with namespace %v", cluster.Name, key.Name)
				continue
			}
			clog.Error("get namespace %v from cluster %v failed: %v", key.Name, cluster.Name, err)
			return nil, err
		}

		// if namespace is tenant hnc
		if t, ok := ns.Labels[constants.TenantLabel]; ok {
			isRelated, err = isRelateWith(t, cli, constants.HncTenantDepth, ctx)
			if err != nil {
				clog.Error("judge relationship of cluster % v and namespace %v failed: %v", cluster.Name, key.Name, err)
				return nil, err
			}
		}

		// if namespace is project hnc
		if p, ok := ns.Labels[constants.ProjectLabel]; ok {
			isRelated, err = isRelateWith(p, cli, constants.HncProjectDepth, ctx)
			if err != nil {
				clog.Error("judge relationship of cluster % v and namespace %v failed: %v", cluster.Name, key.Name, err)
				return nil, err
			}
		}

		// add related cluster to result
		if isRelated {
			clusterNames = append(clusterNames, cluster.Name)
		}
	}

	return clusterNames, nil
}

// getClustersByProject get related clusters by given project
func getClustersByProject(ctx context.Context, project string) (*clusterv1.ClusterList, error) {
	var clusterItem []clusterv1.Cluster

	projectLabel := constants.ProjectNsPrefix + project + constants.HncSuffix
	labelSelector, err := labels.Parse(fmt.Sprintf("%v=%v", projectLabel, "1"))
	if err != nil {
		return nil, err
	}

	clusters := multicluster.Interface().FuzzyCopy()
	for _, cluster := range clusters {
		cli := cluster.Client.Cache()
		nsList := corev1.NamespaceList{}
		err := cli.List(ctx, &nsList, &client.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			return nil, err
		}
		// this cluster is related with project if we found any namespaces under given project
		if len(nsList.Items) > 0 {
			clusterItem = append(clusterItem, *cluster.RawCluster)
		}
	}

	return &clusterv1.ClusterList{Items: clusterItem}, nil
}

func getAssignedResource(cli mgrclient.Client, cluster string) (cpu resource.Quantity, mem resource.Quantity, gpu resource.Quantity, err error) {
	labelSelector, err := labels.Parse(fmt.Sprintf("%v=%v", constants.ClusterLabel, cluster))
	if err != nil {
		return resource.Quantity{}, resource.Quantity{}, resource.Quantity{}, err
	}

	listObjs := v1.CubeResourceQuotaList{}
	err = cli.Direct().List(context.Background(), &listObjs, &client.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return resource.Quantity{}, resource.Quantity{}, resource.Quantity{}, err
	}

	for _, obj := range listObjs.Items {
		hard := obj.Spec.Hard
		if v, ok := hard[corev1.ResourceLimitsCPU]; ok {
			cpu.Add(v)
		}
		if v, ok := hard[corev1.ResourceLimitsMemory]; ok {
			mem.Add(v)
		}
		if v, ok := hard[quota.ResourceNvidiaGPU]; ok {
			gpu.Add(v)
		}
	}
	return cpu, mem, gpu, err
}
