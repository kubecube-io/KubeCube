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
	tenantv1 "github.com/kubecube-io/kubecube/pkg/apis/tenant/v1"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/authorization"
	"github.com/kubecube-io/kubecube/pkg/utils/meta"
	"k8s.io/apimachinery/pkg/selection"
	"sort"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
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

type clusterInfoOpts struct {
	statusFilter      string
	nodeLabelSelector labels.Selector
	pruneInfo         bool
	pageNum           int
	pageSize          int
}

// makeClusterInfos make cluster info with clusters given
func makeClusterInfos(ctx context.Context, clusters clusterv1.ClusterList, opts clusterInfoOpts) ([]clusterInfo, error) {
	// populate cluster info one by one
	infos := make([]clusterInfo, 0)
	memberClusterInfos := make([]clusterInfo, 0)
	for _, item := range clusters.Items {
		info := clusterInfo{}
		clusterName := item.Name

		// populate metadata of cluster
		info.clusterMetaInfo = makeMetadataInfo(item)

		// set cluster status abnormal if we do not have it or not receive heartbeat
		internalCluster, err := multicluster.Interface().Get(clusterName)
		if internalCluster == nil || err != nil {
			info.clusterMetaInfo.Status = string(clusterv1.ClusterAbnormal)
		}

		// filter by query status param
		if len(opts.statusFilter) == 0 || info.Status == opts.statusFilter {
			if item.Spec.IsMemberCluster {
				memberClusterInfos = append(memberClusterInfos, info)
			} else {
				infos = append(infos, info)
			}
		}
	}

	// sort up pivot clusters by create time
	sort.SliceStable(infos, func(i, j int) bool {
		return infos[i].CreateTime.After(infos[j].CreateTime)
	})

	// sort up member clusters by create time
	sort.SliceStable(memberClusterInfos, func(i, j int) bool {
		return memberClusterInfos[i].CreateTime.After(memberClusterInfos[j].CreateTime)
	})

	// append member cluster infos behind pivot clusters to keep pivot clusters at first
	infos = append(infos, memberClusterInfos...)

	if opts.pageNum > 0 && opts.pageSize > 0 {
		// paginate result
		infos = paginateClusterInfos(infos, opts.pageNum, opts.pageSize)
	}

	// make livedata after paginate to reduce io query
	if !opts.pruneInfo {
		start := time.Now()
		needLiveDataNum := 0
		wg := &sync.WaitGroup{}
		for i, info := range infos {
			if info.clusterMetaInfo.Status == string(clusterv1.ClusterAbnormal) {
				// do not populate livedata if abnormal
				continue
			}
			internalCluster, err := multicluster.Interface().Get(info.ClusterName)
			if err != nil {
				clog.Warn("continue make livedata cause cluster %v abnormal: %v", info.ClusterName, err)
				continue
			}

			wg.Add(1)
			needLiveDataNum++

			// we use goroutine to process livedata by concurrency.
			// note: it doesn't need lock here cause there is no race when write data to different index place.
			go func(cli mgrclient.Client, clusterName string, index int) {
				defer wg.Done()
				livedataInfo, err := makeLivedataInfo(ctx, cli, clusterName, opts)
				if err != nil {
					clog.Warn("make livedata failed for cluster %v cause %v", clusterName, err)
					return
				}
				infos[index].clusterLivedataInfo = livedataInfo
			}(internalCluster.Client, info.ClusterName, i)
		}
		wg.Wait()
		clog.Info("make livedata for cluster len(%v) cost %v", needLiveDataNum, time.Now().Sub(start))
	}

	return infos, nil
}

func paginateClusterInfos(infos []clusterInfo, pageNum int, pageSize int) []clusterInfo {
	start := (pageNum - 1) * pageSize
	end := pageNum * pageSize
	if start > len(infos) {
		return nil
	}
	if end > len(infos) {
		end = len(infos)
	}
	return infos[start:end]
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

// makeMetadataInfo just get metadata of cluster.
func makeMetadataInfo(cluster clusterv1.Cluster) clusterMetaInfo {
	info := clusterMetaInfo{}

	state := cluster.Status.State
	if state == nil {
		processState := clusterv1.ClusterProcessing
		state = &processState
	}

	// set up cluster meta info
	info.ClusterName = cluster.Name
	info.Status = string(*state)
	info.ClusterDescription = cluster.Spec.Description
	info.CreateTime = cluster.CreationTimestamp.Time
	info.IsMemberCluster = cluster.Spec.IsMemberCluster
	info.IsWritable = cluster.Spec.IsWritable
	info.HarborAddr = cluster.Spec.HarborAddr
	info.KubeApiServer = cluster.Spec.KubernetesAPIEndpoint
	info.NetworkType = cluster.Spec.NetworkType
	info.IngressDomainSuffix = cluster.Spec.IngressDomainSuffix
	info.Labels = cluster.Labels
	info.Annotations = cluster.Annotations

	if info.Annotations == nil {
		info.Annotations = make(map[string]string)
	}

	// set cluster cn name same as en name by default
	if _, ok := info.Annotations[constants.CubeCnAnnotation]; !ok {
		info.Annotations[constants.CubeCnAnnotation] = cluster.Name
	}

	return info
}

// makeLivedataInfo populate livedata of cluster, sometimes be slow.
func makeLivedataInfo(ctx context.Context, cli mgrclient.Client, cluster string, opts clusterInfoOpts) (clusterLivedataInfo, error) {
	info := clusterLivedataInfo{}

	// populate nodes used resources info by metrics api
	metricListCtx, cancel := context.WithTimeout(ctx, time.Second)
	nodesMc, err := cli.Metrics().MetricsV1beta1().NodeMetricses().List(metricListCtx, metav1.ListOptions{LabelSelector: opts.nodeLabelSelector.String()})
	if err != nil {
		// record error from metric server, but ensure return normal
		clog.Warn("get cluster %v nodes metrics failed: %v", cluster, err)
	} else {
		for _, m := range nodesMc.Items {
			info.UsedCPU += int(m.Usage.Cpu().MilliValue())                                           // 1000 m
			info.UsedMem += convertUnit(m.Usage.Memory().String(), strproc.Mi)                        // 1024 Mi
			info.UsedStorage += convertUnit(m.Usage.Storage().String(), strproc.Mi)                   // 1024 Mi
			info.UsedStorageEphemeral += convertUnit(m.Usage.StorageEphemeral().String(), strproc.Mi) // 1024 Mi
		}
	}

	// releases resources if call metric api completes before timeout elapses
	// or any errors occurred
	cancel()

	// populate node resources info
	nodes := corev1.NodeList{}
	err = cli.Cache().List(ctx, &nodes, &client.ListOptions{LabelSelector: opts.nodeLabelSelector})
	if err != nil {
		return info, fmt.Errorf("get cluster %v nodes failed: %v", cluster, err)
	}

	info.NodeCount = len(nodes.Items)

	for _, n := range nodes.Items {
		info.TotalCPU += int(n.Status.Capacity.Cpu().MilliValue())                                           // 1000 m
		info.TotalMem += convertUnit(n.Status.Capacity.Memory().String(), strproc.Mi)                        // 1024 Mi
		info.TotalStorage += convertUnit(n.Status.Capacity.Storage().String(), strproc.Mi)                   // 1024 Mi
		info.TotalStorageEphemeral += convertUnit(n.Status.Capacity.StorageEphemeral().String(), strproc.Mi) // 1024 Mi
	}

	ns := corev1.NamespaceList{}
	err = cli.Cache().List(ctx, &ns)
	if err != nil {
		return info, fmt.Errorf("get cluster %v namespace failed: %v", cluster, err)
	}

	info.NamespaceCount = len(ns.Items)

	podList := &corev1.PodList{}
	err = cli.Cache().List(context.TODO(), podList)
	if err != nil {
		return info, err
	}
	nodesName := sets.NewString()
	for i := range nodes.Items {
		nodesName.Insert(nodes.Items[i].Name)
	}

	for i := range podList.Items {
		statusPhase := podList.Items[i].Status.Phase
		if nodesName.Has(podList.Items[i].Spec.NodeName) && statusPhase != corev1.PodSucceeded && statusPhase != corev1.PodFailed {
			req, limit := podRequestsAndLimits(&podList.Items[i])
			cpuReq, cpuLimit, memoryReq, memoryLimit := req[corev1.ResourceCPU], limit[corev1.ResourceCPU], req[corev1.ResourceMemory], limit[corev1.ResourceMemory]
			info.UsedCPURequest += int(cpuReq.MilliValue())                    // 1000 m
			info.UsedCPULimit += int(cpuLimit.MilliValue())                    // 1000 m
			info.UsedMemRequest += convertUnit(memoryReq.String(), strproc.Mi) // 1024 Mi
			info.UsedMemLimit += convertUnit(memoryLimit.String(), strproc.Mi) // 1024 Mi
		}
	}

	return info, nil
}

func convertUnit(data, expectedUnit string) int {
	value, err := strproc.BinaryUnitConvert(data, expectedUnit)
	if err != nil {
		// error should not occur here
		clog.Warn(err.Error())
	}

	// note: decimal point will be truncated.
	return int(value)
}

// isRelateWith return true if third level namespace exist under of ancestor namespace
func isRelateWith(namespace string, cli cache.Cache, depth string, ctx context.Context) (bool, error) {
	if depth == constants.HncCurrentDepth {
		return true, nil
	}

	hncLabel := namespace + constants.HncSuffix
	nsList := corev1.NamespaceList{}

	selector, err := labels.Parse(fmt.Sprintf("%v=%v", hncLabel, depth))
	if err != nil {
		return false, err
	}

	err = cli.List(ctx, &nsList, &client.ListOptions{LabelSelector: selector})
	if err != nil {
		return false, err
	}

	if len(nsList.Items) > 0 {
		return true, nil
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
			continue
		}

		// example: kubecube-tenant-tenant1.tree.hnc.x-k8s.io/depth: "0"
		if depth, ok := ns.Labels[namespace+constants.HncSuffix]; ok && depth == constants.HncCurrentDepth {
			if strings.HasPrefix(namespace, constants.TenantNsPrefix) {
				// if namespace is tenant hnc
				isRelated, err = isRelateWith(namespace, cli, constants.HncTenantDepth, ctx)
				if err != nil {
					clog.Error("judge relationship of cluster % v and namespace %v failed: %v", cluster.Name, key.Name, err)
					continue
				}
			} else if strings.HasPrefix(namespace, constants.ProjectNsPrefix) {
				// if namespace is project hnc
				isRelated, err = isRelateWith(namespace, cli, constants.HncProjectDepth, ctx)
				if err != nil {
					clog.Error("judge relationship of cluster % v and namespace %v failed: %v", cluster.Name, key.Name, err)
					continue
				}
			}
		}

		// add related cluster to result
		if isRelated {
			clusterNames = append(clusterNames, cluster.Name)
		}
	}

	return clusterNames, nil
}

// filterClustersByProject get related clusters by given project
func filterClustersByProject(ctx context.Context, clusterList clusterv1.ClusterList, project string) (clusterv1.ClusterList, error) {
	var clusterItem []clusterv1.Cluster

	projectLabel := constants.ProjectNsPrefix + project + constants.HncSuffix
	labelSelector, err := labels.Parse(fmt.Sprintf("%v=%v", projectLabel, "1"))
	if err != nil {
		return clusterv1.ClusterList{}, err
	}

	for _, cluster := range clusterList.Items {
		cli, err := multicluster.Interface().GetClient(cluster.Name)
		if err != nil {
			// ignore unhealthy error
			clog.Warn(err.Error())
		}
		if cli == nil {
			continue
		}
		nsList := corev1.NamespaceList{}
		err = cli.Cache().List(ctx, &nsList, &client.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			return clusterv1.ClusterList{}, err
		}
		// this cluster is related with project if we found any namespaces under given project
		if len(nsList.Items) > 0 {
			clusterItem = append(clusterItem, cluster)
		}
	}

	return clusterv1.ClusterList{Items: clusterItem}, nil
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
		if v, ok := hard[corev1.ResourceRequestsCPU]; ok {
			cpu.Add(v)
		}
		if v, ok := hard[corev1.ResourceRequestsMemory]; ok {
			mem.Add(v)
		}
		if v, ok := hard[quota.ResourceNvidiaGPU]; ok {
			gpu.Add(v)
		}
	}
	return cpu, mem, gpu, err
}

func listAllHncNsFunc(ctx context.Context) func(cli mgrclient.Client) (corev1.NamespaceList, error) {
	return func(cli mgrclient.Client) (corev1.NamespaceList, error) {
		nsLIst := corev1.NamespaceList{}
		labelSelector, err := labels.Parse(constants.HncTenantLabel)
		if err != nil {
			return nsLIst, err
		}
		err = cli.Cache().List(ctx, &nsLIst, &client.ListOptions{LabelSelector: labelSelector})
		return nsLIst, err
	}
}

func listHncNsByTenantsFunc(ctx context.Context, tenantList []string) func(cli mgrclient.Client) (corev1.NamespaceList, error) {
	return func(cli mgrclient.Client) (corev1.NamespaceList, error) {
		nsLIst := corev1.NamespaceList{}
		for _, tenant := range tenantList {
			tempNsList := corev1.NamespaceList{}
			labelSelector, err := labels.Parse(fmt.Sprintf("%v=%v", constants.HncTenantLabel, tenant))
			if err != nil {
				return tempNsList, err
			}
			err = cli.Cache().List(ctx, &tempNsList, &client.ListOptions{LabelSelector: labelSelector})
			if err != nil {
				return tempNsList, err
			}
			nsLIst.Items = append(nsLIst.Items, tempNsList.Items...)
		}
		return nsLIst, nil
	}
}

func getVisibleTenants(ctx context.Context, cli mgrclient.Client, userName string, tenants []string) ([]string, []tenantv1.Tenant, error) {
	visibleTenants, err := authorization.GetVisibleTenants(ctx, cli, userName)
	if err != nil {
		return nil, nil, err
	}
	visibleTenantsSet := sets.NewString()
	for _, t := range visibleTenants {
		visibleTenantsSet.Insert(t.Name)
	}
	if len(tenants) == 0 {
		return visibleTenantsSet.UnsortedList(), visibleTenants, nil
	}
	queryTenantSet := sets.NewString(tenants...)
	if !visibleTenantsSet.IsSuperset(queryTenantSet) {
		return nil, nil, fmt.Errorf("query tenants (%v) is not visible for user (%v)", queryTenantSet.UnsortedList(), userName)
	}
	queryTenantsCr := []tenantv1.Tenant{}
	for _, tenant := range visibleTenants {
		if queryTenantSet.Has(tenant.Name) {
			queryTenantsCr = append(queryTenantsCr, tenant)
		}
	}
	return queryTenantSet.UnsortedList(), queryTenantsCr, nil
}

type clusterDate struct {
	cnName string
	state  *clusterv1.ClusterState
}

func listCubeResourceQuota(ctx context.Context, cli mgrclient.Client, tenants []string, tenantsCr []tenantv1.Tenant, clusters []string) ([]cubeResourceQuotaData, error) {
	ls := labels.NewSelector()
	r1, err := labels.NewRequirement(constants.ClusterLabel, selection.In, clusters)
	if err != nil {
		return nil, err
	}
	r2, err := labels.NewRequirement(constants.TenantLabel, selection.In, tenants)
	if err != nil {
		return nil, err
	}
	ls = ls.Add(*r1)
	ls = ls.Add(*r2)

	list := v1.CubeResourceQuotaList{}
	err = cli.Cache().List(ctx, &list, &client.ListOptions{LabelSelector: ls})
	if err != nil {
		return nil, err
	}

	// construct cube resource quota map
	quotaMap := make(map[string]v1.CubeResourceQuota, len(list.Items))
	for _, v := range list.Items {
		meta.TrimObjectMeta(&v)
		quotaMap[v.Name] = v
	}

	// construct cluster cn name map
	clusterMap := make(map[string]clusterDate, len(clusters))
	clusterList := clusterv1.ClusterList{}
	err = cli.Cache().List(ctx, &clusterList)
	if err != nil {
		return nil, err
	}
	for _, v := range clusterList.Items {
		if v.Annotations != nil {
			c := clusterDate{state: v.Status.State}
			cnName, ok := v.Annotations[constants.CubeCnAnnotation]
			if ok {
				c.cnName = cnName
			}
			clusterMap[v.Name] = c
		}
	}

	res := make([]cubeResourceQuotaData, 0, len(tenants)*len(clusters))
	for _, tenant := range tenantsCr {
		for _, cluster := range clusters {
			v := cubeResourceQuotaData{
				ClusterIdentity:   cluster,
				ClusterName:       cluster,
				Tenant:            tenant.Name,
				TenantName:        tenant.Spec.DisplayName,
				CubeResourceQuota: nil,
				ExclusiveNodeHard: nil,
			}
			quotaName := strings.Join([]string{cluster, tenant.Name}, ".")
			q, ok := quotaMap[quotaName]
			if ok {
				v.CubeResourceQuota = &q
			}
			data, ok := clusterMap[cluster]
			if ok {
				v.ClusterName = data.cnName
			}
			v.ClusterState = *data.state
			clusterCli := clients.Interface().Kubernetes(cluster)
			if clusterCli == nil {
				return nil, fmt.Errorf("cluster %v not found", cluster)
			}
			v.ExclusiveNodeHard, err = getExclusiveNodeHard(clusterCli, tenant.Name)
			if err != nil {
				return nil, err
			}
			res = append(res, v)
		}
	}
	return res, nil
}

func getExclusiveNodeHard(cli mgrclient.Client, tenant string) (map[string]corev1.ResourceList, error) {
	ls, err := labels.Parse(fmt.Sprintf("%v=%v", constants.LabelNodeTenant, tenant))
	if err != nil {
		return nil, err
	}

	nodeList := corev1.NodeList{}
	err = cli.Cache().List(context.Background(), &nodeList, &client.ListOptions{LabelSelector: ls})
	if err != nil {
		return nil, err
	}
	ex := make(map[string]corev1.ResourceList, len(nodeList.Items))
	for _, v := range nodeList.Items {
		ex[v.Name] = v.Status.Capacity
	}
	return ex, nil
}

func sortCubeResourceQuotas(qs []cubeResourceQuotaData) []cubeResourceQuotaData {
	sort.SliceStable(qs, func(i, j int) bool {
		return qs[i].Tenant+qs[i].ClusterName < qs[j].Tenant+qs[j].ClusterName
	})

	res := []cubeResourceQuotaData{}
	bothUnsetted := []cubeResourceQuotaData{}
	oneUnsetted := []cubeResourceQuotaData{}

	for _, v := range qs {
		switch {
		case len(v.ExclusiveNodeHard) == 0 && v.CubeResourceQuota == nil:
			bothUnsetted = append(bothUnsetted, v)
		case len(v.ExclusiveNodeHard) == 0 || v.CubeResourceQuota == nil:
			oneUnsetted = append(oneUnsetted, v)
		default:
			res = append(res, v)
		}
	}
	res = append(res, oneUnsetted...)
	res = append(res, bothUnsetted...)
	return res
}
