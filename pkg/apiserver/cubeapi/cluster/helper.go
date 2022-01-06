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

	v1 "github.com/kubecube-io/kubecube/pkg/apis/quota/v1"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/quota"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	mgrclient "github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/strproc"
)

const (
	// Namespace depth is relative to current namespace depth.
	// Example:
	// tenant-1
	// └── [s] project-1
	//	   └── [s] ns-1
	// ns-1 namespace has three depth label:
	// 1. ns-1.tree.hnc.x-k8s.io/depth: "0"
	// 2. project-1.tree.hnc.x-k8s.io/depth: "1"
	// 3. tenant-1.tree.hnc.x-k8s.io/depth: "2"
	currentDepth = "0"
	projectDepth = "1"
	tenantDepth  = "2"

	// hncSuffix record depth of namespace in HNC
	hncSuffix = ".tree.hnc.x-k8s.io/depth"

	// hncAnnotation must exist in sub namespace
	hncAnnotation = "hnc.x-k8s.io/subnamespace-of"
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
				info.UsedCPU += strproc.Str2int(m.Usage.Cpu().String())/1000000 + 1
				info.UsedMem += strproc.Str2int(m.Usage.Memory().String()) / 1024
				info.UsedStorage += (strproc.Str2int(m.Usage.Storage().String()) + 1) / 1024
				info.UsedStorageEphemeral += (strproc.Str2int(m.Usage.StorageEphemeral().String()) + 1) / 1024
			}
		}

		nodes := corev1.NodeList{}
		err = cli.Cache().List(ctx, &nodes)
		if err != nil {
			return nil, fmt.Errorf("get cluster %v nodes failed: %v", v, err)
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
			return nil, fmt.Errorf("get cluster %v namespace failed: %v", v, err)
		}

		info.NamespaceCount = len(ns.Items)

		if len(statusFilter) == 0 {
			infos = append(infos, info)
		} else if info.Status == statusFilter {
			infos = append(infos, info)
		}
	}

	return infos, nil
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
	if depth == currentDepth {
		return true, nil
	}

	hncLabel := namespace + hncSuffix
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
			isRelated, err = isRelateWith(t, cli, tenantDepth, ctx)
			if err != nil {
				clog.Error("judge relationship of cluster % v and namespace %v failed: %v", cluster.Name, key.Name, err)
				return nil, err
			}
		}

		// if namespace is project hnc
		if p, ok := ns.Labels[constants.ProjectLabel]; ok {
			isRelated, err = isRelateWith(p, cli, projectDepth, ctx)
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
