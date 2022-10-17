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
	"regexp"
	"unicode/utf8"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/domain"
)

var (
	clusterLog clog.CubeLogger
	//clusterClient client.Client
)

type ClusterValidator struct {
	clusterv1.Cluster
}

func NewClusterValidator(mgrClient client.Client) *ClusterValidator {
	clusterLog = clog.WithName("Webhook").WithName("ClusterValidator")
	//clusterClient = mgrClient
	return &ClusterValidator{}
}

func (c *ClusterValidator) GetObjectKind() schema.ObjectKind {

	return c
}

func (c *ClusterValidator) DeepCopyObject() runtime.Object {
	return &ClusterValidator{}
}

func (c *ClusterValidator) ValidateCreate() error {
	log := clusterLog.WithValues("ValidateCreate", c.Name)
	log.Debug("Create validate start")
	err := generateValidate(c.Cluster)
	if err != nil {
		return err
	}

	if err = domain.ValidatorDomainSuffix([]string{c.Spec.IngressDomainSuffix}); err != nil {
		return err
	}
	log.Debug("Create validate success")
	return nil
}

func (c *ClusterValidator) ValidateUpdate(old runtime.Object) error {
	log := clusterLog.WithValues("ValidateUpdate", c.Name)
	log.Debug("Update validate start")
	err := generateValidate(c.Cluster)
	if err != nil {
		return err
	}

	if err = domain.ValidatorDomainSuffix([]string{c.Spec.IngressDomainSuffix}); err != nil {
		return err
	}
	log.Debug("Update validate success")
	return nil
}

func (c *ClusterValidator) ValidateDelete() error {

	return nil
}

const qnameCharFmt string = "[A-Za-z0-9\u4e00-\u9fa5]"
const qnameExtCharFmt string = "[-A-Za-z0-9_\u4e00-\u9fa5]"
const qualifiedNameFmt string = "(" + qnameCharFmt + qnameExtCharFmt + "*)?" + qnameCharFmt
const annotationCnValueErrMsg string = "must consist of alphanumeric or chinese characters, '-', '_' or '.', and must start and end with an alphanumeric or chinese character"
const annotationCnValueLength int = 100
const annotationCnValueFmt string = "(" + qualifiedNameFmt + ")?"

var annotationCnValueRegexp = regexp.MustCompile("^" + annotationCnValueFmt + "$")

// generateValidate validate properties of cluster
func generateValidate(cluster clusterv1.Cluster) error {
	cnName, ok := cluster.GetAnnotations()[constants.CubeCnAnnotation]
	if ok {
		if err := isValidCnName(cnName); err != nil {
			return err
		}
	}

	return nil
}

func isValidCnName(value string) error {
	// can not use len to count cause chinese
	if utf8.RuneCountInString(value) > annotationCnValueLength {
		return fmt.Errorf(validation.MaxLenError(annotationCnValueLength))
	}

	if !annotationCnValueRegexp.MatchString(value) {
		return fmt.Errorf(validation.RegexError(annotationCnValueErrMsg, annotationCnValueFmt, "计算集群1", "member_集群-1", "123"))
	}

	return nil
}
