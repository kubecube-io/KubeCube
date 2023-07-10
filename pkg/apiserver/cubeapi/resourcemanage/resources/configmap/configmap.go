/*
Copyright 2023 KubeCube Authors

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

package configmap

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kubecube-io/kubecube/pkg/utils/audit"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"

	resourcemanage "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/handle"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources"
	"github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/enum"
	"github.com/kubecube-io/kubecube/pkg/clients"
	mgrclient "github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
)

func init() {
	resourcemanage.SetExtendHandler(enum.ConfigMapType, handle)
}

type extenderPerReq struct {
	ginContext *gin.Context
	client     mgrclient.Client
	cluster    string
	namespace  string
	name       string
}

func handle(extendCtx resourcemanage.ExtendContext) (interface{}, *errcode.ErrorInfo) {
	access := resources.NewSimpleAccess(extendCtx.Cluster, extendCtx.Username, extendCtx.Namespace)
	if allow := access.AccessAllow("", "configmaps", "create"); !allow {
		return nil, errcode.ForbiddenErr
	}
	cli := clients.Interface().Kubernetes(extendCtx.Cluster)
	if cli == nil {
		return nil, errcode.ClusterNotFoundError(extendCtx.Cluster)
	}
	ext := extenderPerReq{
		client:     cli,
		ginContext: extendCtx.GinContext,
		cluster:    extendCtx.Cluster,
		name:       extendCtx.ResourceName,
		namespace:  extendCtx.Namespace,
	}
	return ext.processReq()
}

func (e *extenderPerReq) processReq() (_ interface{}, errInfo *errcode.ErrorInfo) {
	switch e.ginContext.Request.Method {
	case http.MethodPost:
		return e.processCreate()
	case http.MethodPut:
		return e.processUpdate()
	case http.MethodDelete:
		return e.processDelete()
	}

	return nil, errcode.NotFoundErr
}

func (e *extenderPerReq) processCreate() (_ interface{}, errInfo *errcode.ErrorInfo) {
	c := e.ginContext
	cli := e.client

	cm := &v1.ConfigMap{}
	if err := c.ShouldBindJSON(cm); err != nil {
		return nil, errcode.InvalidBodyFormat
	}

	cm.Namespace = e.namespace

	if err := cli.Direct().Create(c.Request.Context(), cm); err != nil {
		return nil, errcode.BadRequest(err)
	}

	audit.SetAuditInfo(c, audit.CreateConfigMap, generateCmName(cm.Name, cm.Namespace, e.cluster, string(cm.UID)), cm)

	return nil, nil
}

func (e *extenderPerReq) processUpdate() (_ interface{}, errInfo *errcode.ErrorInfo) {
	c := e.ginContext
	cli := e.client

	toUpdateCm := &v1.ConfigMap{}
	err := c.ShouldBindJSON(toUpdateCm)
	if err != nil {
		return nil, errcode.BadRequest(err)
	}

	ctx := c.Request.Context()

	toUpdateCm.Name = e.name
	toUpdateCm.Namespace = e.namespace

	var uid types.UID

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		newCm := &v1.ConfigMap{}
		err := cli.Direct().Get(ctx, types.NamespacedName{Name: toUpdateCm.Name, Namespace: toUpdateCm.Namespace}, newCm)
		if err != nil {
			return err
		}

		uid = newCm.UID

		newCm.Labels = toUpdateCm.Labels
		newCm.Annotations = toUpdateCm.Annotations
		newCm.Data = toUpdateCm.Data
		newCm.Finalizers = toUpdateCm.Finalizers
		newCm.BinaryData = toUpdateCm.BinaryData
		newCm.Immutable = toUpdateCm.Immutable

		return cli.Direct().Update(ctx, newCm)
	})
	if err != nil {
		return nil, errcode.BadRequest(err)
	}

	audit.SetAuditInfo(c, audit.UpdateConfigMap, generateCmName(toUpdateCm.Name, toUpdateCm.Namespace, e.cluster, string(uid)), toUpdateCm)

	return nil, nil
}

func (e *extenderPerReq) processDelete() (_ interface{}, errInfo *errcode.ErrorInfo) {
	c := e.ginContext
	ctx := c.Request.Context()
	cmName := e.name
	cmNamespace := e.namespace
	cluster := e.cluster

	if len(cmName) == 0 || len(cmNamespace) == 0 {
		return nil, errcode.BadRequest(nil)
	}

	cli := clients.Interface().Kubernetes(cluster)
	if cli == nil {
		return nil, errcode.ClusterNotFoundError(cluster)
	}

	toDeleteCm := &v1.ConfigMap{}
	err := cli.Direct().Get(ctx, types.NamespacedName{Name: cmName, Namespace: cmNamespace}, toDeleteCm)
	if err != nil && errors.IsNotFound(err) {
		return nil, errcode.BadRequest(err)
	}

	if err := cli.Direct().Delete(ctx, toDeleteCm); err != nil {
		return nil, errcode.BadRequest(err)
	}

	audit.SetAuditInfo(c, audit.DeleteConfigMap, generateCmName(toDeleteCm.Name, toDeleteCm.Namespace, cluster, string(toDeleteCm.UID)), toDeleteCm)

	return nil, nil
}

// generateCmName will generate configmap name by {name}/{namespace}/{cluster}/{uid}
func generateCmName(name, namespace, cluster, uid string) string {
	return name + "/" + namespace + "/" + cluster + "/" + uid
}
