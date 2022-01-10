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

package job

import (
	"context"

	jsoniter "github.com/json-iterator/go"
	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources"
	"github.com/kubecube-io/kubecube/pkg/clog"
	mgrclient "github.com/kubecube-io/kubecube/pkg/multicluster/client"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type Job struct {
	ctx       context.Context
	client    mgrclient.Client
	namespace string
	filter    resources.Filter
}

func NewJob(client mgrclient.Client, namespace string, filter resources.Filter) Job {
	ctx := context.Background()
	return Job{
		ctx:       ctx,
		client:    client,
		namespace: namespace,
		filter:    filter,
	}
}

// get extend deployments
func (j *Job) GetExtendJobs() resources.K8sJson {
	resultMap := make(resources.K8sJson)

	// get deployment list from k8s cluster
	var jobList batchv1.JobList
	err := j.client.Cache().List(j.ctx, &jobList, client.InNamespace(j.namespace))
	if err != nil {
		clog.Error("can not find job in %s from cluster, %v", j.namespace, err)
		return nil
	}
	resultMap["total"] = len(jobList.Items)

	// filter list by selector/sort/page
	jobListJson, err := json.Marshal(jobList)
	if err != nil {
		clog.Error("convert deploymentList to json fail, %v", err)
		return nil
	}
	jobListJson = j.filter.FilterResult(jobListJson)
	err = json.Unmarshal(jobListJson, &jobList)
	if err != nil {
		clog.Error("convert json to deploymentList fail, %v", err)
		return nil
	}

	// add pod status info
	resultList := j.addExtendInfo(jobList)

	resultMap["items"] = resultList

	return resultMap
}

func (j *Job) addExtendInfo(jobList batchv1.JobList) resources.K8sJsonArr {
	resultList := make(resources.K8sJsonArr, 0)

	for _, job := range jobList.Items {
		// parse job status
		status := ParseJobStatus(job)

		extendInfo := make(resources.K8sJson)
		extendInfo["status"] = status

		// create result map
		result := make(resources.K8sJson)
		result["metadata"] = job.ObjectMeta
		result["spec"] = job.Spec
		result["status"] = job.Status
		result["extendInfo"] = extendInfo
		resultList = append(resultList, result)
	}

	return resultList
}

func ParseJobStatus(job batchv1.Job) (status string) {
	status = "Running"
	jobStatus := job.Status
	if job.Status.Conditions == nil || len(job.Status.Conditions) == 0 {
		if jobStatus.Active == 0 || jobStatus.Succeeded == 0 || jobStatus.Failed == 0 {
			status = "Pending"
			return
		}
	}
	for _, condition := range jobStatus.Conditions {
		if string(condition.Type) == "Complete" && string(condition.Status) == "True" {
			status = "Complete"
			return
		} else if string(condition.Type) == "Failed" && string(condition.Status) == "True" {
			status = "Failed"
			return
		}
	}
	return
}
