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
	"errors"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcemanage "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/handle"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/enum"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"
	mgrclient "github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"github.com/kubecube-io/kubecube/pkg/utils/filter"
)

type Job struct {
	ctx       context.Context
	client    mgrclient.Client
	namespace string
	filter    *filter.Filter
}

func init() {
	resourcemanage.SetExtendHandler(enum.JobResourceType, Handle)
}

func Handle(param resourcemanage.ExtendParams) (interface{}, error) {
	access := resources.NewSimpleAccess(param.Cluster, param.Username, param.Namespace)
	if allow := access.AccessAllow("batch", "jobs", "list"); !allow {
		return nil, errors.New(errcode.ForbiddenErr.Message)
	}
	kubernetes := clients.Interface().Kubernetes(param.Cluster)
	if kubernetes == nil {
		return nil, errors.New(errcode.ClusterNotFoundError(param.Cluster).Message)
	}
	job := NewJob(kubernetes, param.Namespace, param.Filter)
	return job.GetExtendJobs()
}

func NewJob(client mgrclient.Client, namespace string, filter *filter.Filter) Job {
	ctx := context.Background()
	return Job{
		ctx:       ctx,
		client:    client,
		namespace: namespace,
		filter:    filter,
	}
}

// GetExtendJobs get extend deployments
func (j *Job) GetExtendJobs() (*unstructured.Unstructured, error) {
	resultMap := make(map[string]interface{})

	// get deployment list from k8s cluster
	var jobList batchv1.JobList
	err := j.client.Cache().List(j.ctx, &jobList, client.InNamespace(j.namespace))
	if err != nil {
		clog.Error("can not find job in %s from cluster, %v", j.namespace, err)
		return nil, err
	}

	// filter list by selector/sort/page
	total, err := j.filter.FilterObjectList(&jobList)
	if err != nil {
		clog.Error("filter jobList error, err: %s", err.Error())
		return nil, err
	}

	// add pod status info
	resultList := j.addExtendInfo(jobList)

	resultMap["total"] = total
	resultMap["items"] = resultList

	return &unstructured.Unstructured{
		Object: resultMap,
	}, nil
}

func (j *Job) addExtendInfo(jobList batchv1.JobList) []unstructured.Unstructured {
	resultList := make([]unstructured.Unstructured, 0)

	for _, job := range jobList.Items {
		// parse job status
		status := ParseJobStatus(job)

		extendInfo := make(map[string]interface{})
		extendInfo["status"] = status

		// create result map
		result := make(map[string]interface{})
		result["metadata"] = job.ObjectMeta
		result["spec"] = job.Spec
		result["status"] = job.Status
		result["extendInfo"] = extendInfo
		resultList = append(resultList, unstructured.Unstructured{Object: result})
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
