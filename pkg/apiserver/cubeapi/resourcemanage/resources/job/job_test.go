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
package job_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/job"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	"github.com/kubecube-io/kubecube/pkg/multicluster/client/fake"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
)

var _ = Describe("Job", func() {
	var (
		ns      = "namespace-test"
		job1    batchv1.Job
		job2    batchv1.Job
		jobList batchv1.JobList
	)
	BeforeEach(func() {
		job1 = batchv1.Job{
			TypeMeta:   metav1.TypeMeta{Kind: "Job", APIVersion: "batch/v1"},
			ObjectMeta: metav1.ObjectMeta{Name: "job1", Namespace: ns, UID: "jobid1"},
			Status: batchv1.JobStatus{
				Conditions: []batchv1.JobCondition{
					{Type: "Complete", Status: "True"},
				},
			},
		}
		job2 = batchv1.Job{
			TypeMeta:   metav1.TypeMeta{Kind: "Job", APIVersion: "batch/v1"},
			ObjectMeta: metav1.ObjectMeta{Name: "job2", Namespace: ns, UID: "jobid2"},
			Status:     batchv1.JobStatus{Active: 0},
		}
		jobList = batchv1.JobList{Items: []batchv1.Job{job1, job2}}
	})
	JustBeforeEach(func() {
		scheme := runtime.NewScheme()
		corev1.AddToScheme(scheme)
		batchv1.AddToScheme(scheme)
		opts := &fake.Options{
			Scheme:               scheme,
			Objs:                 []client.Object{},
			ClientSetRuntimeObjs: []runtime.Object{},
			Lists:                []client.ObjectList{&jobList},
		}
		multicluster.InitFakeMultiClusterMgrWithOpts(opts)
		clients.InitCubeClientSetWithOpts(nil)
	})

	It("test get job extend info", func() {
		client := clients.Interface().Kubernetes(constants.LocalCluster)
		Expect(client).NotTo(BeNil())
		job := job.NewJob(client, ns, resources.Filter{Limit: 10})
		ret := job.GetExtendJobs()
		Expect(ret["total"]).To(Equal(2))
		items := ret["items"].([]interface{})
		s := items[0].(map[string]interface{})["extendInfo"].(map[string]interface{})["status"]
		Expect(s).To(Equal("Complete"))
		s = items[1].(map[string]interface{})["extendInfo"].(map[string]interface{})["status"]
		Expect(s).To(Equal("Pending"))

	})
})
