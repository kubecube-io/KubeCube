/*
Copyright 2022 KubeCube Authors

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
	"fmt"
	clusterv1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	clusterLog    clog.CubeLogger
	clusterClient client.Client
)

type ClusterValidator struct {
	clusterv1.Cluster
}

func NewClusterValidator(mgrClient client.Client) *ClusterValidator {
	clusterLog = clog.WithName("Webhook").WithName("ClusterValidator")
	clusterClient = mgrClient
	return &ClusterValidator{}
}

func (c *ClusterValidator) GetObjectKind() schema.ObjectKind {

	return c.GetObjectKind()
}

func (c *ClusterValidator) DeepCopyObject() runtime.Object {
	return &ClusterValidator{}
}

func (c *ClusterValidator) ValidateCreate() error {
	log := clusterLog.WithValues("ValidateCreate", c.Name)
	log.Debug("Create validate start")
	if err := validatorDomainSuffix(c, log); err != nil {
		return err
	}
	log.Debug("Create validate success")
	return nil
}

func (c *ClusterValidator) ValidateUpdate(old runtime.Object) error {
	log := clusterLog.WithValues("ValidateUpdate", c.Name)
	log.Debug("Update validate start")
	if err := validatorDomainSuffix(c, log); err != nil {
		return err
	}
	log.Debug("Update validate success")
	return nil
}

func (c *ClusterValidator) ValidateDelete() error {

	return nil
}

func validatorDomainSuffix(c *ClusterValidator, log clog.CubeLogger) error {
	domainSuffix := c.Spec.IngressDomainSuffix
	if len(domainSuffix) != 0 {
		if errs := validation.IsDNS1123Subdomain(domainSuffix); len(errs) > 0 {
			log.Debug("Invalid value: %s ", domainSuffix)
			return fmt.Errorf("Invalid value: %s : a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')\n)", domainSuffix)
		}
	}
	return nil
}
