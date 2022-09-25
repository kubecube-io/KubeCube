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

package cronjob

import (
	"context"
	"errors"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcemanage "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/handle"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/enum"
	jobRes "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/job"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"
	mgrclient "github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"github.com/kubecube-io/kubecube/pkg/utils/filter"
)

type CronJob struct {
	ctx       context.Context
	client    mgrclient.Client
	namespace string
	filter    *filter.Filter
}

func init() {
	resourcemanage.SetExtendHandler(enum.CronResourceType, Handle)
}

func Handle(param resourcemanage.ExtendParams) (interface{}, error) {
	access := resources.NewSimpleAccess(param.Cluster, param.Username, param.Namespace)
	if allow := access.AccessAllow("batch", "cronjobs", "list"); !allow {
		return nil, errors.New(errcode.ForbiddenErr.Message)
	}
	kubernetes := clients.Interface().Kubernetes(param.Cluster)
	if kubernetes == nil {
		return nil, errors.New(errcode.ClusterNotFoundError(param.Cluster).Message)
	}
	cronjob := NewCronJob(kubernetes, param.Namespace, param.Filter)
	if param.ResourceName == "" {
		return cronjob.GetExtendCronJobs()
	} else {
		return cronjob.GetExtendCronJob(param.ResourceName)
	}
}

func NewCronJob(client mgrclient.Client, namespace string, filter *filter.Filter) CronJob {
	ctx := context.Background()
	return CronJob{
		ctx:       ctx,
		client:    client,
		namespace: namespace,
		filter:    filter,
	}
}

// GetExtendCronJobs get extend deployments
func (c *CronJob) GetExtendCronJobs() (*unstructured.Unstructured, error) {
	resultMap := make(map[string]interface{})

	// get deployment list from k8s cluster
	var cronJobList batchv1beta1.CronJobList
	err := c.client.Cache().List(c.ctx, &cronJobList, client.InNamespace(c.namespace))
	if err != nil {
		clog.Error("can not find cronjob in %s from cluster, %v", c.namespace, err)
		return nil, err
	}

	// filter list by selector/sort/page
	total, err := c.filter.FilterObjectList(&cronJobList)
	if err != nil {
		clog.Error("can not filter cronjob, err: %s", err.Error())
		return nil, err
	}
	// add pod status info
	resultList := c.addExtendInfo(cronJobList)

	resultMap["total"] = total
	resultMap["items"] = resultList

	return &unstructured.Unstructured{Object: resultMap}, nil
}

// GetExtendCronJob get extend deployments
func (c *CronJob) GetExtendCronJob(name string) (*unstructured.Unstructured, error) {
	// get deployment list from k8s cluster
	var cronJob batchv1beta1.CronJob
	err := c.client.Cache().Get(c.ctx, types.NamespacedName{Namespace: c.namespace, Name: name}, &cronJob)
	if err != nil {
		clog.Error("can not find cronjob %s/%s from cluster, %v", c.namespace, name, err)
		return nil, err
	}

	var cronJobList batchv1beta1.CronJobList
	cronJobList.Items = []batchv1beta1.CronJob{cronJob}
	resultList := c.addExtendInfo(cronJobList)
	if len(resultList) == 0 {
		return nil, fmt.Errorf("can not parse cronjob %s/%s", c.namespace, name)
	}

	return &resultList[0], err
}

func (c *CronJob) addExtendInfo(cronJobList batchv1beta1.CronJobList) []unstructured.Unstructured {
	resultList := make([]unstructured.Unstructured, 0)
	jobArrMap := c.getOwnerJobs()
	for _, cronJob := range cronJobList.Items {
		// parse job status
		status := parseCronJobStatus(cronJob)
		jobArr, ok := jobArrMap[string(cronJob.UID)]
		runningJobCount := 0
		if ok {
			for _, job := range jobArr {
				extendInfo := job.(map[string]interface{})["extendInfo"]
				extendInfoStatus := extendInfo.(map[string]interface{})["status"].(string)
				if extendInfoStatus == "Running" {
					runningJobCount++
				}
			}
		}
		extendInfo := make(map[string]interface{})
		extendInfo["status"] = status
		extendInfo["runningJobCount"] = runningJobCount
		extendInfo["jobCount"] = len(jobArr)
		extendInfo["jobs"] = jobArr

		// create result map
		result := make(map[string]interface{})
		result["metadata"] = cronJob.ObjectMeta
		result["spec"] = cronJob.Spec
		result["status"] = cronJob.Status
		result["extendInfo"] = extendInfo
		res := unstructured.Unstructured{
			Object: result,
		}
		resultList = append(resultList, res)
	}

	return resultList
}

func (c *CronJob) getOwnerJobs() map[string][]interface{} {
	result := make(map[string][]interface{})
	var jobList batchv1.JobList
	err := c.client.Cache().List(c.ctx, &jobList, client.InNamespace(c.namespace))
	if err != nil {
		clog.Error("can not find jobs from cluster, %v", err)
		return nil
	}

	for _, job := range jobList.Items {
		if len(job.OwnerReferences) == 0 {
			continue
		}
		uid := string(job.OwnerReferences[0].UID)

		status := jobRes.ParseJobStatus(job)
		extendInfo := make(map[string]interface{})
		extendInfo["status"] = status
		// create result map
		jobMap := make(map[string]interface{})
		jobMap["metadata"] = job.ObjectMeta
		jobMap["spec"] = job.Spec
		jobMap["status"] = job.Status
		jobMap["extendInfo"] = extendInfo

		if jobArr, ok := result[uid]; ok {
			jobArr = append(jobArr, jobMap)
			result[uid] = jobArr
		} else {
			var jobArrTemp []interface{}
			jobArrTemp = append(jobArrTemp, jobMap)
			result[uid] = jobArrTemp
		}
	}
	return result
}

func parseCronJobStatus(cronjob batchv1beta1.CronJob) (status string) {
	if *cronjob.Spec.Suspend {
		return "Fail"
	}
	return "Running"
}
