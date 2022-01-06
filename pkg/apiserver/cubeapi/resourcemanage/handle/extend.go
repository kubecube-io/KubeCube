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

package resourcemanage

import (
	"io/ioutil"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	cronjobRes "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/cronjob"
	deploymentRes "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/deployment"
	jobRes "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/job"
	podRes "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/pod"
	podlogRes "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/podlog"
	pvcRes "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/pvc"
	serviceRes "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/service"
	"github.com/kubecube-io/kubecube/pkg/authentication/authenticators/token"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"github.com/kubecube-io/kubecube/pkg/utils/response"
)

// api/v1/cube/extend/clusters/{cluster}/namespaces/{namespace}/{resourceType}
func ExtendHandle(c *gin.Context) {
	// request param
	cluster := c.Param("cluster")
	namespace := c.Param("namespace")
	resourceType := c.Param("resourceType")
	resourceName := c.Param("resourceName")
	filter := parseQueryParams(c)
	httpMethod := c.Request.Method
	// k8s client
	client := clients.Interface().Kubernetes(cluster)
	if client == nil {
		response.FailReturn(c, errcode.ClusterNotFoundError(cluster))
		return
	}
	// access
	username := ""
	userInfo, err := token.GetUserFromReq(c.Request)
	if err == nil {
		username = userInfo.Username
	}
	access := resources.NewSimpleAccess(cluster, username, namespace)

	switch resourceType {
	case "deployments":
		if allow := access.AccessAllow("apps", "deployments", "list"); !allow {
			response.FailReturn(c, errcode.ForbiddenErr)
			return
		}
		deployment := deploymentRes.NewDeployment(client, namespace, filter)
		result := deployment.GetExtendDeployments()
		response.SuccessReturn(c, result)
	case "pods":
		if allow := access.AccessAllow("", "pods", "list"); !allow {
			response.FailReturn(c, errcode.ForbiddenErr)
			return
		}
		pod := podRes.NewPod(client, namespace, filter)
		result := pod.GetPods()
		if result == nil {
			response.FailReturn(c, errcode.ServerErr)
			return
		}
		response.SuccessReturn(c, result)
	case "services":
		if allow := access.AccessAllow("apps", "services", "list"); !allow {
			response.FailReturn(c, errcode.ForbiddenErr)
			return
		}
		service := serviceRes.NewService(client, namespace, filter)
		result := service.GetExtendServices()
		response.SuccessReturn(c, result)
	case "externalAccess":
		externalAccess := serviceRes.NewExternalAccess(client, namespace, resourceName, filter)
		if httpMethod == http.MethodGet {
			if allow := access.AccessAllow("", "services", "list"); !allow {
				response.FailReturn(c, errcode.ForbiddenErr)
				return
			}
			result, err := externalAccess.GetExternalAccess()
			if err != nil {
				response.FailReturn(c, errcode.DealError(err))
				return
			}
			response.SuccessReturn(c, result)
			return
		} else if httpMethod == http.MethodPost {
			if allow := access.AccessAllow("", "services", "create"); !allow {
				response.FailReturn(c, errcode.ForbiddenErr)
				return
			}
			body, err := ioutil.ReadAll(c.Request.Body)
			if err != nil {
				response.FailReturn(c, errcode.InvalidBodyFormat)
				return
			}
			err = externalAccess.SetExternalAccess(body)
			if err != nil {
				response.FailReturn(c, errcode.DealError(err))
				return
			}
			response.SuccessReturn(c, "success")
			return
		} else {
			response.FailReturn(c, errcode.InvalidHttpMethod)
			return
		}
	case "externalAccessAddress":
		if allow := access.AccessAllow("", "services", "list"); !allow {
			response.FailReturn(c, errcode.ForbiddenErr)
			return
		}
		externalAccess := serviceRes.NewExternalAccess(client, namespace, resourceName, filter)
		if httpMethod == http.MethodGet {
			result := externalAccess.GetExternalIP()
			response.SuccessReturn(c, result)
			return
		}
	case "jobs":
		if allow := access.AccessAllow("batch", "jobs", "list"); !allow {
			response.FailReturn(c, errcode.ForbiddenErr)
			return
		}
		job := jobRes.NewJob(client, namespace, filter)
		result := job.GetExtendJobs()
		response.SuccessReturn(c, result)
	case "cronjobs":
		if allow := access.AccessAllow("batch", "cronjobs", "list"); !allow {
			response.FailReturn(c, errcode.ForbiddenErr)
			return
		}
		if resourceName == "" {
			cronjob := cronjobRes.NewCronJob(client, namespace, filter)
			result := cronjob.GetExtendCronJobs()
			response.SuccessReturn(c, result)
		} else {
			cronjob := cronjobRes.NewCronJob(client, namespace, filter)
			result := cronjob.GetExtendCronJob(resourceName)
			response.SuccessReturn(c, result)
		}
	case "pvcworkloads":
		if allow := access.AccessAllow("", "persistentvolumeclaims", "list"); !allow {
			response.FailReturn(c, errcode.ForbiddenErr)
			return
		}
		pvc := pvcRes.NewPvc(client, namespace, filter)
		result := pvc.GetPvcWorkloads(resourceName)
		response.SuccessReturn(c, result)
	case "logs":
		if allow := access.AccessAllow("", "pods", "list"); !allow {
			response.FailReturn(c, errcode.ForbiddenErr)
			return
		}
		podlog := podlogRes.NewPodLog(client, namespace, filter)
		podlog.HandleLogs(c)
	default:
		response.FailReturn(c, errcode.InvalidResourceTypeErr)
	}
}

// GetFeatureConfig shows layout of integrated components
// all users have read-only access ability
func GetFeatureConfig(c *gin.Context) {
	cli := clients.Interface().Kubernetes(constants.LocalCluster)
	if cli == nil {
		response.FailReturn(c, errcode.InternalServerError)
		return
	}

	cm := &corev1.ConfigMap{}
	key := types.NamespacedName{Name: "kubecube-feature-config", Namespace: constants.CubeNamespace}

	err := cli.Cache().Get(c.Request.Context(), key, cm)
	if err != nil {
		if errors.IsNotFound(err) {
			response.FailReturn(c, errcode.CustomReturn(http.StatusNotFound, "configmap(%v/%v) not found", key.Namespace, key.Name))
			return
		}
		response.FailReturn(c, errcode.InternalServerError)
		return
	}

	response.SuccessReturn(c, cm.Data)
}

// GetConfigMap show system configMap
// all users have read-only access ability
func GetConfigMap(c *gin.Context) {
	cmName := c.Param("configmap")
	if cmName == "" {
		response.FailReturn(c, errcode.InvalidBodyFormat)
		return
	}

	cli := clients.Interface().Kubernetes(constants.LocalCluster)
	if cli == nil {
		response.FailReturn(c, errcode.InternalServerError)
		return
	}

	cm := &corev1.ConfigMap{}
	key := types.NamespacedName{Name: cmName, Namespace: constants.CubeNamespace}

	err := cli.Cache().Get(c.Request.Context(), key, cm)
	if err != nil {
		if errors.IsNotFound(err) {
			response.FailReturn(c, errcode.CustomReturn(http.StatusNotFound, "configmap(%v/%v) not found", key.Namespace, key.Name))
			return
		}
		response.FailReturn(c, errcode.InternalServerError)
		return
	}

	response.SuccessReturn(c, cm.Data)
}
