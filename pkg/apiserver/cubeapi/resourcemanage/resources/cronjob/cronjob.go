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

	jsoniter "github.com/json-iterator/go"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources"
	jobRes "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/job"
	"github.com/kubecube-io/kubecube/pkg/clog"
	mgrclient "github.com/kubecube-io/kubecube/pkg/multicluster/client"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type CronJob struct {
	ctx       context.Context
	client    mgrclient.Client
	namespace string
	filter    resources.Filter
}

func NewCronJob(client mgrclient.Client, namespace string, filter resources.Filter) CronJob {
	ctx := context.Background()
	return CronJob{
		ctx:       ctx,
		client:    client,
		namespace: namespace,
		filter:    filter,
	}
}

// get extend deployments
func (c *CronJob) GetExtendCronJobs() resources.K8sJson {
	resultMap := make(resources.K8sJson)

	// get deployment list from k8s cluster
	var cronJobList batchv1beta1.CronJobList
	err := c.client.Cache().List(c.ctx, &cronJobList, client.InNamespace(c.namespace))
	if err != nil {
		clog.Error("can not find cronjob in %s from cluster, %v", c.namespace, err)
		return nil
	}
	resultMap["total"] = len(cronJobList.Items)

	// filter list by selector/sort/page
	jobListJson, err := json.Marshal(cronJobList)
	if err != nil {
		clog.Error("convert deploymentList to json fail, %v", err)
		return nil
	}
	jobListJson = c.filter.FilterResult(jobListJson)
	err = json.Unmarshal(jobListJson, &cronJobList)
	if err != nil {
		clog.Error("convert json to deploymentList fail, %v", err)
		return nil
	}

	// add pod status info
	resultList := c.addExtendInfo(cronJobList)

	resultMap["items"] = resultList

	return resultMap
}

// get extend deployments
func (c *CronJob) GetExtendCronJob(name string) resources.K8sJson {

	// get deployment list from k8s cluster
	var cronJob batchv1beta1.CronJob
	err := c.client.Cache().Get(c.ctx, types.NamespacedName{Namespace: c.namespace, Name: name}, &cronJob)
	if err != nil {
		clog.Error("can not find cronjob %s/%s from cluster, %v", c.namespace, name, err)
		return nil
	}

	var cronJobList batchv1beta1.CronJobList
	cronJobList.Items = []batchv1beta1.CronJob{cronJob}
	resultList := c.addExtendInfo(cronJobList)
	if len(resultList) == 0 {
		clog.Error("can not parse cronjob %s/%s", c.namespace, name)
		return nil
	}

	return resultList[0].(resources.K8sJson)
}

func (c *CronJob) addExtendInfo(cronJobList batchv1beta1.CronJobList) resources.K8sJsonArr {
	resultList := make(resources.K8sJsonArr, 0)
	jobArrMap := c.getOwnerJobs()
	for _, cronJob := range cronJobList.Items {
		// parse job status
		status := parseCronJobStatus(cronJob)
		jobArr, ok := jobArrMap[string(cronJob.UID)]
		runningJobCount := 0
		if ok {
			for _, job := range jobArr {
				extendInfo := job.(resources.K8sJson)["extendInfo"]
				extendInfoStatus := extendInfo.(resources.K8sJson)["status"].(string)
				if extendInfoStatus == "Running" {
					runningJobCount++
				}
			}
		}
		extendInfo := make(resources.K8sJson)
		extendInfo["status"] = status
		extendInfo["runningJobCount"] = runningJobCount
		extendInfo["jobCount"] = len(jobArr)
		extendInfo["jobs"] = jobArr

		// create result map
		result := make(resources.K8sJson)
		result["metadata"] = cronJob.ObjectMeta
		result["spec"] = cronJob.Spec
		result["status"] = cronJob.Status
		result["extendInfo"] = extendInfo
		resultList = append(resultList, result)
	}

	return resultList
}

func (c *CronJob) getOwnerJobs() map[string]resources.K8sJsonArr {
	result := make(map[string]resources.K8sJsonArr)
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
		extendInfo := make(resources.K8sJson)
		extendInfo["status"] = status
		// create result map
		jobMap := make(resources.K8sJson)
		jobMap["metadata"] = job.ObjectMeta
		jobMap["spec"] = job.Spec
		jobMap["status"] = job.Status
		jobMap["extendInfo"] = extendInfo

		if jobArr, ok := result[uid]; ok {
			jobArr = append(jobArr, jobMap)
			result[uid] = jobArr
		} else {
			var jobArrTemp resources.K8sJsonArr
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
