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
	"encoding/base64"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	userinfo "k8s.io/apiserver/pkg/authentication/user"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/hierarchical-namespaces/api/v1alpha2"

	clusterv1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
	"github.com/kubecube-io/kubecube/pkg/authorizer/rbac"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	mgrclient "github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"github.com/kubecube-io/kubecube/pkg/quota"
	"github.com/kubecube-io/kubecube/pkg/utils/access"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"github.com/kubecube-io/kubecube/pkg/utils/kubeconfig"
	"github.com/kubecube-io/kubecube/pkg/utils/response"
)

const subPath = "/clusters"

func (h *handler) AddApisTo(root *gin.Engine) {
	r := root.Group(constants.ApiPathRoot + subPath)
	r.GET("info", h.getClusterInfo)
	r.GET("/:cluster/monitor", h.getClusterMonitorInfo)
	r.GET("/:cluster/livedata", h.getClusterLivedata)
	r.GET("namespaces", h.getClusterNames)
	r.GET("resources", h.getClusterResource)
	r.GET("subnamespaces", h.getSubNamespaces)
	r.POST("register", h.registerCluster)
	r.POST("add", h.addCluster)
	r.POST("nsquota", h.createNsAndQuota)
}

type result struct {
	Total int           `json:"total"`
	Items []clusterInfo `json:"items"`
}

// clusterInfo contains meta info and livedata info
type clusterInfo struct {
	clusterMetaInfo
	clusterLivedataInfo
}

type clusterMetaInfo struct {
	ClusterName         string            `json:"clusterName"`
	ClusterDescription  string            `json:"clusterDescription"`
	NetworkType         string            `json:"networkType"`
	HarborAddr          string            `json:"harborAddr"`
	IsMemberCluster     bool              `json:"isMemberCluster"`
	IsWritable          bool              `json:"isWritable"`
	CreateTime          time.Time         `json:"createTime"`
	KubeApiServer       string            `json:"kubeApiServer"`
	Status              string            `json:"status"`
	IngressDomainSuffix string            `json:"ingressDomainSuffix,omitempty"`
	Labels              map[string]string `json:"labels,omitempty"`
	Annotations         map[string]string `json:"annotations,omitempty"`
}

type clusterLivedataInfo struct {
	NodeCount             int `json:"nodeCount"`
	NamespaceCount        int `json:"namespaceCount"`
	UsedCPU               int `json:"usedCpu"`
	TotalCPU              int `json:"totalCpu"`
	UsedMem               int `json:"usedMem"`
	TotalMem              int `json:"totalMem"`
	TotalStorage          int `json:"totalStorage"`
	UsedStorage           int `json:"usedStorage"`
	TotalStorageEphemeral int `json:"totalStorageEphemeral"`
	UsedStorageEphemeral  int `json:"usedStorageEphemeral"`
	TotalGpu              int `json:"totalGpu"`
	UsedGpu               int `json:"usedGpu"`
	UsedCPURequest        int `json:"usedCpuRequest"`
	UsedCPULimit          int `json:"usedCpuLimit"`
	UsedMemRequest        int `json:"usedMemRequest"`
	UsedMemLimit          int `json:"usedMemLimit"`
}

type handler struct {
	rbac.Interface
	mgrclient.Client
}

func NewHandler() *handler {
	h := new(handler)
	h.Interface = rbac.NewDefaultResolver(constants.LocalCluster)
	h.Client = clients.Interface().Kubernetes(constants.LocalCluster)
	return h
}

// getClusterInfo get cluster details by cluster name
// @Summary Show cluster info
// @Description get cluster info by cluster name or project name, non query params means all clusters info
// @Tags cluster
// @Param cluster query string false "cluster info search by cluster name"
// @Param project query string false "cluster info search by project name"
// @Param status query string false "cluster info search by cluster status"
// @Success 200 {object} result "{"total":3,"items":[{"clusterName":"member-1","clusterDescription":"this is member cluster","networkType":"calico","harborAddr":"","isMemberCluster":true,"createTime":"2022-05-06T11:33:15+08:00","kubeApiServer":"https://10.173.33.3:6443","status":"normal","nodeCount":1,"namespaceCount":19,"usedCpu":549,"totalCpu":8000,"usedMem":7276,"totalMem":16648,"totalStorage":0,"usedStorage":0,"totalStorageEphemeral":42208,"usedStorageEphemeral":0,"totalGpu":0,"usedGpu":0,"usedCpuRequest":3300,"usedCpuLimit":4200,"usedMemRequest":3874,"usedMemLimit":7265},{"clusterName":"pivot-cluster","clusterDescription":"There is a pivot cluster dating with KubeCube","networkType":"","harborAddr":"","isMemberCluster":false,"createTime":"2022-04-28T14:41:26+08:00","kubeApiServer":"10.173.33.2:6443","status":"normal","nodeCount":1,"namespaceCount":18,"usedCpu":886,"totalCpu":8000,"usedMem":8996,"totalMem":16648,"totalStorage":0,"usedStorage":0,"totalStorageEphemeral":42208,"usedStorageEphemeral":0,"totalGpu":0,"usedGpu":0,"usedCpuRequest":3000,"usedCpuLimit":3900,"usedMemRequest":3469,"usedMemLimit":6860},{"clusterName":"member-2","clusterDescription":"this is member cluster","networkType":"calico","harborAddr":"","isMemberCluster":true,"createTime":"2022-04-28T16:12:13+08:00","kubeApiServer":"10.173.33.4:6443","status":"normal","nodeCount":1,"namespaceCount":19,"usedCpu":929,"totalCpu":8000,"usedMem":7187,"totalMem":16648,"totalStorage":0,"usedStorage":0,"totalStorageEphemeral":42208,"usedStorageEphemeral":0,"totalGpu":0,"usedGpu":0,"usedCpuRequest":3000,"usedCpuLimit":3900,"usedMemRequest":3469,"usedMemLimit":6860}]}"
// @Failure 500 {object} errcode.ErrorInfo
// @Router /api/v1/cube/clusters/info  [get]
func (h *handler) getClusterInfo(c *gin.Context) {
	var (
		cli         = h.Client
		ctx         = c.Request.Context()
		clusterList = clusterv1.ClusterList{}

		pageNum  int
		pageSize int
		err      error
	)

	clusterName := c.Query("cluster")
	clusterStatus := c.Query("status")
	projectName := c.Query("project")
	nodeLabelSelector := c.Query("nodeLabelSelector")
	pruneInfo := c.Query("prune")

	// parse paginate params if had
	if c.Query("pageNum") != "" && c.Query("pageSize") != "" {
		pageNum, err = strconv.Atoi(c.Query("pageNum"))
		if err != nil {
			response.FailReturn(c, errcode.BadRequest(err))
			return
		}
		pageSize, err = strconv.Atoi(c.Query("pageSize"))
		if err != nil {
			response.FailReturn(c, errcode.BadRequest(err))
			return
		}
	}

	start := time.Now()

	switch {
	// find cluster by given name
	case len(clusterName) > 0:
		key := types.NamespacedName{Name: clusterName}
		cluster := clusterv1.Cluster{}
		err := cli.Direct().Get(ctx, key, &cluster)
		if err != nil {
			response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, "get cluster failed: %v", err))
			return
		}
		clusterList.Items = []clusterv1.Cluster{cluster}
	// give back all clusters by default
	default:
		clusters := clusterv1.ClusterList{}
		err := cli.Direct().List(ctx, &clusters)
		if err != nil {
			response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, "list cluster failed: %v", err))
			return
		}
		clusterList = clusters
	}

	clog.Info("list cluster len(%v) cost time: %v", len(clusterList.Items), time.Now().Sub(start))

	if len(projectName) > 0 {
		clusterList, err = filterClustersByProject(ctx, clusterList, projectName)
		if err != nil {
			response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, "get clusters by given project %v failed: %v", projectName, err))
			return
		}
	}

	selector, err := labels.Parse(nodeLabelSelector)
	if err != nil {
		response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, "labels selector invalid: %v", err))
		return
	}

	opts := clusterInfoOpts{
		pruneInfo:         pruneInfo == "true",
		statusFilter:      clusterStatus,
		nodeLabelSelector: selector,
		pageNum:           pageNum,
		pageSize:          pageSize,
	}

	res := result{Total: len(clusterList.Items)}

	infos, err := makeClusterInfos(c.Request.Context(), clusterList, opts)
	if err != nil {
		response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, err.Error()))
		return
	}

	res.Items = infos

	response.SuccessReturn(c, res)
	return
}

type monitorInfo struct {
	NamespaceCount        int `json:"namespaceCount"`
	NodeCount             int `json:"nodeCount"`
	TotalCPU              int `json:"totalCpu"`
	UsedCPU               int `json:"usedCpu"`
	TotalMem              int `json:"totalMem"`
	UsedMem               int `json:"usedMem"`
	TotalStorage          int `json:"totalStorage"`
	UsedStorage           int `json:"usedStorage"`
	TotalStorageEphemeral int `json:"totalStorageEphemeral"`
	UsedStorageEphemeral  int `json:"usedStorageEphemeral"`
	TotalGpu              int `json:"totalGpu"`
	UsedGpu               int `json:"usedGpu"`
}

// getClusterMonitorInfo fetch resource used infos of specified cluster.
// temp not be used
func (h *handler) getClusterMonitorInfo(c *gin.Context) {
	cluster := c.Param("cluster")
	if len(cluster) == 0 {
		response.FailReturn(c, errcode.InvalidBodyFormat)
		return
	}

	info, err := makeMonitorInfo(c.Request.Context(), cluster)
	if err != nil {
		response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, err.Error()))
		return
	}

	response.SuccessReturn(c, info)
}

// getClusterLivedata fetch livedata infos of specified cluster.
// temp not be used
func (h *handler) getClusterLivedata(c *gin.Context) {
	nodeLabelSelector := c.Query("nodeLabelSelector")
	cluster := c.Param("cluster")
	if len(cluster) == 0 {
		response.FailReturn(c, errcode.InvalidBodyFormat)
		return
	}

	selector, err := labels.Parse(nodeLabelSelector)
	if err != nil {
		response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, "labels selector invalid: %v", err))
		return
	}

	cli := clients.Interface().Kubernetes(cluster)
	if cli == nil {
		response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, "cluster %v abnormal", cluster))
		return
	}

	info, err := makeLivedataInfo(c.Request.Context(), cli, cluster, clusterInfoOpts{nodeLabelSelector: selector})
	if err != nil {
		response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, err.Error()))
		return
	}

	response.SuccessReturn(c, info)
}

// getClusterNames get cluster name where the namespace work in
// @Summary Show all clusters bind to namespace
// @Description get cluster name where the namespace work in
// @Tags cluster
// @Param namespace query string false "clusters search by namespace"
// @Success 200 {object} map[string]interface{} "{"items":["member-2","member-1","pivot-cluster"],"total":3}"
// @Failure 500 {object} errcode.ErrorInfo
// @Router /api/v1/cube/clusters/namespaces  [get]
func (h *handler) getClusterNames(c *gin.Context) {
	var (
		namespace    = c.Query("namespace")
		ctx          = c.Request.Context()
		clusterNames []string
	)

	if len(namespace) > 0 {
		clusters, err := getClustersByNamespace(namespace, ctx)
		if err != nil {
			response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, err.Error()))
			return
		}
		clusterNames = clusters
	} else {
		clusterNames = listClusterNames()
	}

	sort.SliceStable(clusterNames, func(i, j int) bool {
		return clusterNames[i] < clusterNames[j]
	})

	res := map[string]interface{}{
		"total": len(clusterNames),
		"items": clusterNames,
	}

	response.SuccessReturn(c, res)
}

// getClusterResource get allocate resource of cluster
// @Summary Get allocate resource of cluster
// @Description get allocate resource of cluster
// @Tags cluster
// @Param cluster query string true "allocate resource search by cluster"
// @Success 200 {object} map[string]interface{} "{"assignedCpu":"4","assignedGpu":"0","assignedMem":"4000Mi","capacityCpu":"8","capacityGpu":"0","capacityMem":"15876Mi"}"
// @Failure 500 {object} errcode.ErrorInfo
// @Router /api/v1/cube/clusters/resources  [get]
func (h *handler) getClusterResource(c *gin.Context) {
	cluster := c.Query("cluster")
	nodeLabelSelector := c.Query("nodeLabelSelector")
	cli := clients.Interface().Kubernetes(cluster)
	if cli == nil {
		response.FailReturn(c, errcode.ClusterNotFoundError(cluster))
		return
	}

	selector, err := labels.Parse(nodeLabelSelector)
	if err != nil {
		response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, "labels selector invalid: %v", err))
		return
	}

	nodes := v1.NodeList{}
	err = cli.Cache().List(c.Request.Context(), &nodes, &client.ListOptions{LabelSelector: selector})
	if err != nil {
		response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, "get cluster %v nodes failed: %v", cluster, err))
	}

	capacityCpu := quota.ZeroQ()
	capacityMem := quota.ZeroQ()
	capacityGpu := quota.ZeroQ()

	for _, v := range nodes.Items {
		capacityCpu.Add(*v.Status.Capacity.Cpu())
		capacityMem.Add(*v.Status.Capacity.Memory())
		nodeGpu, ok := v.Status.Capacity[constants.GpuNvidia]
		if ok {
			capacityGpu.Add(nodeGpu)
		}
	}

	assignedCpu, assignedMem, assignedGpu, err := getAssignedResource(cli, cluster)
	if err != nil {
		response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, err.Error()))
		return
	}

	res := map[string]interface{}{
		"capacityCpu": capacityCpu,
		"assignedCpu": assignedCpu,
		"assignedMem": assignedMem,
		"capacityGpu": capacityGpu,
		"assignedGpu": assignedGpu,
		"capacityMem": capacityMem,
	}

	response.SuccessReturn(c, res)
}

// getSubNamespaces list sub namespace by tenant
// @Summary Get sub namespace
// @Description get sub namespaces by tenant
// @Tags cluster
// @Param tenant query string false "list sub namespaces by tenant"
// @Success 200 {object} map[string]interface{} "{"items":[{"namespace":"ns-3","cluster":"member-1","project":"project-2","namespaceBody":{"metadata":{"name":"ns-3","uid":"8b557f42-dcda-4555-bfba-4bc448b4d66f","resourceVersion":"862129","creationTimestamp":"2022-04-28T08:18:35Z","labels":{"hnc.x-k8s.io/included-namespace":"true","kubecube-project-project-2.tree.hnc.x-k8s.io/depth":"1","kubecube-tenant-tenant-2.tree.hnc.x-k8s.io/depth":"2","kubecube.hnc.x-k8s.io/project":"project-2","kubecube.hnc.x-k8s.io/tenant":"tenant-2","kubernetes.io/metadata.name":"ns-3","ns-3.tree.hnc.x-k8s.io/depth":"0"},"annotations":{"hnc.x-k8s.io/subnamespace-of":"kubecube-project-project-2"},"managedFields":[{"manager":"manager","operation":"Update","apiVersion":"v1","time":"2022-05-06T03:33:37Z","fieldsType":"FieldsV1","fieldsV1":{"f:metadata":{"f:annotations":{".":{},"f:hnc.x-k8s.io/subnamespace-of":{}},"f:labels":{".":{},"f:kubecube-project-project-2.tree.hnc.x-k8s.io/depth":{},"f:kubecube-tenant-tenant-2.tree.hnc.x-k8s.io/depth":{},"f:kubecube.hnc.x-k8s.io/project":{},"f:kubecube.hnc.x-k8s.io/tenant":{},"f:kubernetes.io/metadata.name":{},"f:ns-3.tree.hnc.x-k8s.io/depth":{}}}}}]},"spec":{"finalizers":["kubernetes"]},"status":{"phase":"Active"}}},{"namespace":"ns-2","cluster":"member-2","project":"project-2","namespaceBody":{"metadata":{"name":"ns-2","uid":"8924ca69-c309-4e94-a55b-893b21ffde17","resourceVersion":"2270","creationTimestamp":"2022-04-28T08:18:23Z","labels":{"hnc.x-k8s.io/included-namespace":"true","kubecube-project-project-2.tree.hnc.x-k8s.io/depth":"1","kubecube-tenant-tenant-2.tree.hnc.x-k8s.io/depth":"2","kubecube.hnc.x-k8s.io/project":"project-2","kubecube.hnc.x-k8s.io/tenant":"tenant-2","kubernetes.io/metadata.name":"ns-2","ns-2.tree.hnc.x-k8s.io/depth":"0"},"annotations":{"hnc.x-k8s.io/subnamespace-of":"kubecube-project-project-2"},"managedFields":[{"manager":"manager","operation":"Update","apiVersion":"v1","time":"2022-04-28T08:18:23Z","fieldsType":"FieldsV1","fieldsV1":{"f:metadata":{"f:annotations":{".":{},"f:hnc.x-k8s.io/subnamespace-of":{}},"f:labels":{".":{},"f:kubecube-project-project-2.tree.hnc.x-k8s.io/depth":{},"f:kubecube-tenant-tenant-2.tree.hnc.x-k8s.io/depth":{},"f:kubecube.hnc.x-k8s.io/project":{},"f:kubecube.hnc.x-k8s.io/tenant":{},"f:kubernetes.io/metadata.name":{},"f:ns-2.tree.hnc.x-k8s.io/depth":{}}}}}]},"spec":{"finalizers":["kubernetes"]},"status":{"phase":"Active"}}}],"total":2}"
// @Failure 500 {object} errcode.ErrorInfo
// @Router /api/v1/cube/clusters/subnamespaces  [get]
func (h *handler) getSubNamespaces(c *gin.Context) {
	type respBody struct {
		Namespace     string       `json:"namespace"`
		Cluster       string       `json:"cluster"`
		ClusterName   string       `json:"clusterName,omitempty"`
		Project       string       `json:"project"`
		Tenant        string       `json:"tenant"`
		NamespaceBody v1.Namespace `json:"namespaceBody"`
	}

	fuzzyName := c.Query("fuzzyname")
	tenantArray := c.Query("tenant")
	tenantList := strings.Split(tenantArray, "|")
	if len(tenantArray) == 0 {
		tenantList = nil
	}
	ctx := c.Request.Context()
	clusters := multicluster.Interface().FuzzyCopy()

	user := c.Query("user")
	if len(user) == 0 {
		user = c.GetString(constants.UserName)
	}

	// list all hnc managed namespaces
	listFunc := func(cli mgrclient.Client) (v1.NamespaceList, error) {
		nsLIst := v1.NamespaceList{}
		labelSelector, err := labels.Parse(constants.HncTenantLabel)
		if err != nil {
			return nsLIst, err
		}
		err = cli.Cache().List(ctx, &nsLIst, &client.ListOptions{LabelSelector: labelSelector})
		return nsLIst, err
	}

	// list hnc managed namespaces by given tenants
	if len(tenantList) > 0 {
		listFunc = func(cli mgrclient.Client) (v1.NamespaceList, error) {
			nsLIst := v1.NamespaceList{}
			for _, tenant := range tenantList {
				tempNsList := v1.NamespaceList{}
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

	items := make([]respBody, 0)

	// search in every cluster
	for _, cluster := range clusters {
		cli := cluster.Client
		nsList, err := listFunc(cli) // these namespaces contain project ns and ns under project
		if err != nil {
			response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, err.Error()))
			return
		}

		for _, ns := range nsList.Items {
			project, ok1 := ns.Labels[constants.HncProjectLabel]
			tenant, ok2 := ns.Labels[constants.HncTenantLabel]
			if ok1 && ok2 && ns.ObjectMeta.DeletionTimestamp.IsZero() {
				// filter project ns(such as kubecube-project-project-1).
				if ns.Labels[constants.ProjectNsPrefix+project+constants.HncSuffix] != constants.HncProjectDepth {
					continue
				}

				if fuzzyName != "" && !strings.Contains(ns.Name, fuzzyName) {
					continue
				}

				// only care about ns under project that the user can see
				// todo: use better way
				allowed, err := rbac.IsAllowResourceAccess(&rbac.DefaultResolver{Cache: cli.Cache()}, user, "pods", constants.GetVerb, constants.ProjectNsPrefix+project)
				if err != nil {
					response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, err.Error()))
					return
				}

				if !allowed {
					continue
				}

				clusterName := cluster.RawCluster.Annotations[constants.CubeCnAnnotation]
				if len(clusterName) == 0 {
					clusterName = cluster.Name
				}
				item := respBody{
					Namespace:     ns.Name,
					Cluster:       cluster.Name,
					ClusterName:   clusterName,
					Project:       project,
					Tenant:        tenant,
					NamespaceBody: ns,
				}

				items = append(items, item)
			}
		}
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].NamespaceBody.CreationTimestamp.Time.After(items[j].NamespaceBody.CreationTimestamp.Time)
	})

	res := map[string]interface{}{
		"total": len(items),
		"items": items,
	}

	response.SuccessReturn(c, res)
}

// scriptData is the data to render script
type scriptData struct {
	ClusterName string `json:"clusterName"`
	KubeConfig  string `json:"kubeConfig"`
	K8sEndpoint string `json:"k8sEndpoint,omitempty"`
	NetworkType string `json:"networkType,omitempty"`
	Description string `json:"description,omitempty"`
	HarborAddr  string `json:"harborAddr,omitempty"`
}

// addCluster return script which need be execute in member cluster node
// @Summary Add cluster
// @Description add cluster to KubeCube
// @Tags cluster
// @Param scriptData body scriptData true "new cluster raw data"
// @Success 200 {string} string "success"
// @Failure 400 {object} errcode.ErrorInfo
// @Failure 500 {object} errcode.ErrorInfo
// @Router /api/v1/cube/clusters/addCluster  [post]
func (h *handler) addCluster(c *gin.Context) {
	const (
		defaultNetworkType = "calico"
		defaultDescription = "this is member cluster"
	)

	d := scriptData{}
	err := c.ShouldBindJSON(&d)
	if err != nil {
		response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, err.Error()))
		return
	}

	if len(d.Description) == 0 {
		d.Description = defaultDescription
	}

	if len(d.NetworkType) == 0 {
		d.NetworkType = defaultNetworkType
	}

	kubeConfig, err := base64.StdEncoding.DecodeString(d.KubeConfig)
	if err != nil {
		response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, "kubeConfig invalid: %v", err))
		return
	}

	config, err := kubeconfig.LoadKubeConfigFromBytes(kubeConfig)
	if err != nil {
		response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, "kubeConfig invalid: %v", err))
		return
	}

	if len(d.K8sEndpoint) == 0 {
		d.K8sEndpoint = config.Host
	}

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: d.ClusterName,
		},
		Spec: clusterv1.ClusterSpec{
			KubeConfig:            kubeConfig,
			KubernetesAPIEndpoint: config.Host,
			IsMemberCluster:       true,
			Description:           d.Description,
		},
	}

	if len(d.NetworkType) > 0 {
		cluster.Spec.NetworkType = d.NetworkType
	}

	if len(d.HarborAddr) > 0 {
		cluster.Spec.HarborAddr = d.HarborAddr
	}

	if access := access.AllowAccess(constants.LocalCluster, c.Request, constants.CreateVerb, cluster); !access {
		clog.Debug("permission check fail")
		response.FailReturn(c, errcode.ForbiddenErr)
		return
	}

	err = h.Direct().Create(c.Request.Context(), cluster)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			clog.Warn(err.Error())
		} else {
			response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, err.Error()))
			return
		}
	}

	response.SuccessJsonReturn(c, "success")
}

// registerCluster is a callback api for add cluster to pivot cluster
func (h *handler) registerCluster(c *gin.Context) {
	cluster := &clusterv1.Cluster{}
	err := c.ShouldBindJSON(cluster)
	if err != nil {
		clog.Error(err.Error())
		response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, err.Error()))
		return
	}

	err = h.Direct().Create(c.Request.Context(), cluster)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			clog.Warn(err.Error())
		} else {
			response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, err.Error()))
			return
		}
	}

	response.SuccessJsonReturn(c, "success")
}

type nsAndQuota struct {
	Cluster            string                       `json:"cluster"`
	SubNamespaceAnchor *v1alpha2.SubnamespaceAnchor `json:"subNamespaceAnchor"`
	ResourceQuota      *v1.ResourceQuota            `json:"resourceQuota"`
}

// createNsAndQuota create quota when rbac was spread to new namespace
// @Summary create subNamespace and resourceQuota
// @Description create subNamespace and resourceQuota
// @Tags cluster
// @Param nsAndQuota body nsAndQuota true "ns and quota data"
// @Success 200 {string} string "success"
// @Failure 400 {object} errcode.ErrorInfo
// @Failure 500 {object} errcode.ErrorInfo
// @Router /api/v1/cube/clusters/nsquota  [post]
func (h *handler) createNsAndQuota(c *gin.Context) {
	data := &nsAndQuota{}
	err := c.ShouldBindJSON(data)
	if err != nil {
		response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, err.Error()))
		return
	}

	if access := access.AllowAccess(data.Cluster, c.Request, constants.CreateVerb, data.SubNamespaceAnchor); !access {
		clog.Debug("permission check fail")
		response.FailReturn(c, errcode.ForbiddenErr)
		return
	}

	if access := access.AllowAccess(data.Cluster, c.Request, constants.CreateVerb, data.ResourceQuota); !access {
		clog.Debug("permission check fail")
		response.FailReturn(c, errcode.ForbiddenErr)
		return
	}
	username := c.GetString(constants.UserName)
	cli := clients.Interface().Kubernetes(data.Cluster)
	ctx := c.Request.Context()

	err = cli.Direct().Create(ctx, data.SubNamespaceAnchor)
	if err != nil {
		response.FailReturn(c, errcode.BadRequest(err))
		return
	}

	rollback := func() {
		err := cli.Direct().Delete(ctx, data.SubNamespaceAnchor)
		if err != nil {
			clog.Error(err.Error())
		}
		err = cli.ClientSet().CoreV1().Namespaces().Delete(ctx, data.SubNamespaceAnchor.Name, metav1.DeleteOptions{})
		if err != nil {
			if !errors.IsNotFound(err) {
				clog.Error(err.Error())
			}
		}
	}

	// wait for namespace created
	err = wait.Poll(200*time.Millisecond, 2*time.Second, func() (done bool, err error) {
		_, err = cli.ClientSet().CoreV1().Namespaces().Get(ctx, data.SubNamespaceAnchor.Name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		} else {
			return true, nil
		}
	})
	if err != nil {
		rollback()
		response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, err.Error()))
		return
	}

	const (
		retryCount    = 20
		retryInterval = 100 * time.Millisecond
	)

	_, clusterRoles, err := h.Interface.RolesFor(&userinfo.DefaultInfo{Name: username}, "")
	if err != nil {
		rollback()
		response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, err.Error()))
		return
	}

	// platform level clusterRole does not need wait rbac spread
	toWait := true
	for _, r := range clusterRoles {
		if v, ok := r.GetLabels()[constants.RoleLabel]; ok {
			if v == "platform" {
				toWait = false
			}
		}
	}

	// wait for rbac resources spread
	count := 0
	for toWait {
		if count == retryCount {
			clog.Warn("wait fo rbac spread by hnc retry exceed %v", retryCount)
			break
		}

		list := &rbacv1.RoleBindingList{}
		err = cli.Direct().List(ctx, list, &client.ListOptions{Namespace: data.SubNamespaceAnchor.Name})
		if err != nil {
			rollback()
			response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, err.Error()))
			return
		}
		if len(list.Items) > 0 {
			break
		}
		count++
		time.Sleep(retryInterval)
	}

	// final action failed would rollback whole action
	err = cli.Direct().Create(ctx, data.ResourceQuota)
	if err != nil {
		rollback()
		response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, err.Error()))
		return
	}

	clog.Debug("user %v create ns %v and resourceQuota %v in cluster %v success",
		username, data.SubNamespaceAnchor.Name, data.ResourceQuota.Name, data.Cluster)

	response.SuccessJsonReturn(c, "success")
}
