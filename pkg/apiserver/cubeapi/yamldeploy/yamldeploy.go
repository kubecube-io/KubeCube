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

package yamldeploy

import (
	"bytes"
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/yaml"

	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	"github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"github.com/kubecube-io/kubecube/pkg/utils/audit"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"github.com/kubecube-io/kubecube/pkg/utils/response"
)

var metadataAccessor = meta.NewAccessor()

type yamlDeployPayload struct {
	Yaml string `json:"yaml"`
}

func Deploy(c *gin.Context) {
	dryRun := c.Query("dryRun")

	username := c.GetString(constants.UserName)

	// get cluster info
	clusterName := c.Param("cluster")
	clusters := multicluster.Interface().FuzzyCopy()
	cluster, ok := clusters[clusterName]
	if !ok {
		response.FailReturn(c, errcode.ClusterNotFoundError(clusterName))
		return
	}

	payload := &yamlDeployPayload{}
	err := c.ShouldBindJSON(payload)
	if err != nil {
		response.FailReturn(c, errcode.BadRequest(err))
		return
	}

	body := []byte(payload.Yaml)

	// split yaml resources by "---"
	objs := bytes.Split(body, []byte("---"))
	for _, obj := range objs {
		// trim spaces and newline
		trimmedObj := bytes.TrimSpace(obj)

		// skip empty object
		if len(trimmedObj) == 0 {
			continue
		}

		bodyJson, err := yaml.YAMLToJSON(trimmedObj)
		if err != nil {
			response.FailReturn(c, errcode.InvalidBodyFormat)
			return
		}
		obj, gvk, err := unstructured.UnstructuredJSONScheme.Decode(bodyJson, nil, nil)
		if err != nil {
			response.FailReturn(c, errcode.InvalidBodyFormat)
			return
		}

		// new RestClient
		restClient, err := NewRestClient(cluster.Config, gvk)
		if err != nil {
			response.FailReturn(c, errcode.BadRequest(err))
			return
		}

		// init mapping
		restMapping, err := InitRestMapper(cluster.Client, gvk)
		if err != nil {
			response.FailReturn(c, errcode.CreateMappingError(err.Error()))
			return
		}

		// get namespace
		namespace, err := metadataAccessor.Namespace(obj)
		if err != nil {
			response.FailReturn(c, errcode.MissNamespaceInObj)
			return
		}

		c = audit.SetAuditInfo(c, audit.YamlDeploy, fmt.Sprintf("%s/%s", namespace, restMapping.Resource.String()))

		// create
		_, err = CreateByRestClient(restClient, restMapping, namespace, dryRun, obj, username)
		if err != nil {
			response.FailReturn(c, errcode.DeployYamlError(err.Error()))
			return
		}
	}

	response.SuccessReturn(c, nil)
}

func CreateByRestClient(restClient *rest.RESTClient, mapping *meta.RESTMapping, namespace string, dryRun string, obj runtime.Object, username string) (runtime.Object, error) {
	options := &metav1.CreateOptions{}
	if dryRun == "true" {
		options.DryRun = []string{metav1.DryRunAll}
	}
	return restClient.Post().
		SetHeader(constants.ImpersonateUserKey, username).
		NamespaceIfScoped(namespace, mapping.Scope.Name() == meta.RESTScopeNameNamespace).
		Resource(mapping.Resource.Resource).
		VersionedParams(options, metav1.ParameterCodec).
		Body(obj).
		Do(context.TODO()).
		Get()
}

func NewRestClient(config *rest.Config, gvk *schema.GroupVersionKind) (*rest.RESTClient, error) {

	config.APIPath = "/apis"
	if gvk.Group == corev1.GroupName {
		config.APIPath = "/api"
	}
	if config.NegotiatedSerializer == nil {
		// This codec factory ensures the resources are not converted. Therefore, resources
		// will not be round-tripped through internal versions. Defaulting does not happen
		// on the client.
		config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	}
	config.ContentConfig = resource.UnstructuredPlusDefaultContentConfig()
	config.GroupVersion = &schema.GroupVersion{Group: gvk.Group, Version: gvk.Version}
	restClient, err := rest.RESTClientFor(config)
	if err != nil {
		clog.Warn("create rest client fail, %v", err)
		return nil, err
	}
	return restClient, nil
}

func InitRestMapper(client client.Client, gvk *schema.GroupVersionKind) (*meta.RESTMapping, error) {

	groupResources, err := restmapper.GetAPIGroupResources(client.ClientSet().Discovery())
	if err != nil {
		clog.Warn("restmapper get api group resources fail, %v", err)
		return nil, err
	}
	mapping, err := restmapper.NewDiscoveryRESTMapper(groupResources).RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		clog.Warn("create rest mapping fail, %v", err)
		return nil, err
	}

	return mapping, nil
}
