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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/scheme"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubecube-io/kubecube/pkg/apiserver/middlewares/audit"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	"github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"github.com/kubecube-io/kubecube/pkg/utils/international"
	"github.com/kubecube-io/kubecube/pkg/utils/response"
)

type Handler struct {
	auditHandler audit.Handler
}

type yamlDeployResources struct {
	Objects []unstructured.Unstructured `json:"objects"`
}

func NewHandler(managers *international.Gi18nManagers) *Handler {
	return &Handler{auditHandler: audit.NewHandler(managers)}
}

func (h *Handler) Deploy(c *gin.Context) {
	dryRun := c.Query("dryRun")
	clusterName := c.Param("cluster")
	username := c.GetString(constants.UserName)

	objs := yamlDeployResources{}
	err := c.ShouldBindJSON(&objs)
	if err != nil {
		clog.Warn(err.Error())
		response.FailReturn(c, errcode.InvalidBodyFormat)
		return
	}

	clusters := multicluster.Interface().FuzzyCopy()
	cluster, ok := clusters[clusterName]
	if !ok {
		response.FailReturn(c, errcode.ClusterNotFoundError(clusterName))
		return
	}

	var errs []error

	for _, obj := range objs.Objects {
		objCopy := obj.DeepCopy()
		err := createResource(cluster, username, objCopy, dryRun)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		clog.Info("Deploy %v/%v of %v success", objCopy.GetName(), objCopy.GetNamespace(), objCopy.GroupVersionKind())
		h.sendDeployedAuditEvent(c, objCopy, clusterName)
	}

	err = utilerrors.NewAggregate(errs)
	if err != nil {
		response.FailReturn(c, errcode.BadRequest(err))
		return
	}

	response.SuccessReturn(c, nil)
}

func (h *Handler) sendDeployedAuditEvent(c *gin.Context, obj *unstructured.Unstructured, clusterName string) {
	ctx := context.Background()

	event := &audit.Event{
		EventName:   "create" + obj.GetKind(),
		Description: h.auditHandler.Translate(ctx, http.MethodPost) + obj.GetKind(),
		ResourceReports: []audit.Resource{{
			ResourceType: strings.ToLower(obj.GetKind()),
			ResourceName: fmt.Sprintf("%s/%s/%s/%s", obj.GetName(), obj.GetNamespace(), clusterName, obj.GetUID()),
		}},
		RequestMethod:    http.MethodPost,
		ResponseStatus:   http.StatusOK,
		Url:              c.Request.URL.String(),
		ResponseElements: "success",
	}

	body, err := json.Marshal(obj)
	if err != nil {
		clog.Warn("json marshal failed for %v: %v", event.EventName, err)
		return
	}

	event.RequestParameters = audit.ConsistParameters(c, body)

	// no-block sending
	go h.auditHandler.SendEvent(c, event, &audit.Options{Translate: false})
}

func createResource(cluster *multicluster.FuzzyCluster, username string, obj ctrlclient.Object, dryRun string) error {
	unstructuredObj := obj.(*unstructured.Unstructured)
	gvk := unstructuredObj.GroupVersionKind()

	// new rest client for gvk
	restClient, err := NewRestClient(cluster.Config, &gvk)
	if err != nil {
		return err
	}

	// get rest mapper form cache
	restMapping, err := initRestMapper(cluster.Client, &gvk)
	if err != nil {
		return err
	}

	if _, err := createByRestClient(restClient, restMapping, unstructuredObj.GetNamespace(), dryRun, unstructuredObj, username); err != nil {
		return err
	}

	return nil
}

func createByRestClient(restClient *rest.RESTClient, mapping *meta.RESTMapping, namespace string, dryRun string, obj runtime.Object, username string) (runtime.Object, error) {
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

func initRestMapper(client client.Client, gvk *schema.GroupVersionKind) (*meta.RESTMapping, error) {
	mapping, err := client.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}

	return mapping, nil
}
