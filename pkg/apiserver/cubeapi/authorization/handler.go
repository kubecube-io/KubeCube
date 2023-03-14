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

package authorization

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	userinfo "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubecube-io/kubecube/pkg/authorizer/mapping"
	"github.com/kubecube-io/kubecube/pkg/authorizer/rbac"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"
	mgrclient "github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"github.com/kubecube-io/kubecube/pkg/utils/access"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/env"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"github.com/kubecube-io/kubecube/pkg/utils/response"
)

const subPath = "/authorization"

func (h *handler) AddApisTo(root *gin.Engine) {
	r := root.Group(constants.ApiPathRoot + subPath)
	r.GET("roles", h.getRolesByUser)
	r.GET("clusterroles", h.getClusterRolesByLevel)
	r.GET("users", h.getUsersByRole)
	r.GET("tenants", h.getTenantByUser)
	r.GET("projects", h.getProjectByUser)
	r.GET("identities", h.getIdentity)
	r.POST("bindings", h.createBinds)
	r.DELETE("bindings", h.deleteBinds)
	r.POST("access", h.authorization)
	r.POST("resources", h.resourcesGate)
	r.GET("authitems/:clusterrole", h.getAuthItems)
	r.GET("authitems", h.getAuthItemsByLabelSelector)
	r.POST("authitems", h.setAuthItems)
	r.POST("authitems/permissions", h.getPermissions)
}

type result struct {
	Total int         `json:"total"`
	Items interface{} `json:"items"`
}

type handler struct {
	rbac.Interface
	mgrclient.Client
	cmData map[string]string
}

func NewHandler() *handler {
	h := new(handler)
	h.Interface = rbac.NewDefaultResolver(constants.LocalCluster)
	h.Client = clients.Interface().Kubernetes(constants.LocalCluster)

	cm := corev1.ConfigMap{}
	nn := types.NamespacedName{Name: constants.AuthMappingCM, Namespace: env.CubeNamespace()}
	err := h.Client.Direct().Get(context.Background(), nn, &cm)
	if err != nil {
		clog.Warn("get auth item configmap %v failed: %v", nn, err)
	}

	h.cmData = cm.Data

	return h
}

// getAuthItemsByLabelSelector get auth items by label selector.
func (h *handler) getAuthItemsByLabelSelector(c *gin.Context) {
	labelSelector := c.Query("labelSelector")
	verbose := c.Query("verbose")

	selector, err := labels.Parse(labelSelector)
	if err != nil {
		response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, "labels selector invalid: %v", err))
		return
	}

	clusterRoleList := &rbacv1.ClusterRoleList{}
	err = h.Cache().List(context.Background(), clusterRoleList, &client.ListOptions{LabelSelector: selector})
	if err != nil {
		clog.Error(err.Error())
		response.FailReturn(c, errcode.InternalServerError)
		return
	}

	res := make([]*mapping.RoleAuthBody, 0, len(clusterRoleList.Items))

	for _, clusterRole := range clusterRoleList.Items {
		v := mapping.ClusterRoleMapping(clusterRole.DeepCopy(), h.cmData, verbose == "true")
		res = append(res, v)
	}

	response.SuccessReturn(c, res)
}

// getAuthItems get auth items by ClusterRole name.
func (h *handler) getAuthItems(c *gin.Context) {
	clusterRoleName := c.Param("clusterrole")
	verbose := c.Query("verbose")

	clusterRole := &rbacv1.ClusterRole{}

	err := h.Cache().Get(context.Background(), types.NamespacedName{Name: clusterRoleName}, clusterRole)
	if err != nil {
		if errors.IsNotFound(err) {
			response.FailReturn(c, errcode.CustomReturn(http.StatusNotFound, err.Error()))
			return
		}
		clog.Error(err.Error())
		response.FailReturn(c, errcode.InternalServerError)
		return
	}

	roleAuths := mapping.ClusterRoleMapping(clusterRole, h.cmData, verbose == "true")

	response.SuccessReturn(c, roleAuths)
}

// setAuthItems transfer auth item to ClusterRole into k8s.
func (h *handler) setAuthItems(c *gin.Context) {
	body := &mapping.RoleAuthBody{}
	if err := c.ShouldBindJSON(body); err != nil {
		response.FailReturn(c, errcode.InvalidBodyFormat)
		return
	}

	if len(body.ClusterRoleName) == 0 || len(body.AuthItems) == 0 {
		response.FailReturn(c, errcode.InvalidBodyFormat)
		return
	}

	clusterRole := mapping.RoleAuthMapping(body, h.cmData)
	runtimeObject := clusterRole.DeepCopy()
	_, err := controllerruntime.CreateOrUpdate(context.Background(), h.Client.Direct(), runtimeObject, func() error {
		runtimeObject.Rules = clusterRole.Rules
		return nil
	})
	if err != nil {
		clog.Error(err.Error())
		response.FailReturn(c, errcode.InternalServerError)
		return
	}

	response.SuccessReturn(c, nil)
}

type authItemAccessInfos struct {
	Cluster string `json:"cluster"`
	User    string `json:"user,omitempty"`
	Infos   []struct {
		AuthItem  string `json:"authItem"`
		Namespace string `json:"namespace"`
	} `json:"infos"`
}

// getPermissions query access permissions by asking k8s.
func (h *handler) getPermissions(c *gin.Context) {
	data := &authItemAccessInfos{}
	err := c.ShouldBindJSON(data)
	if err != nil {
		clog.Error(err.Error())
		response.FailReturn(c, errcode.InvalidBodyFormat)
		return
	}

	if len(data.User) == 0 {
		data.User = c.GetString(constants.UserName)
	}

	res := make(map[string]mapping.VerbRepresent)

	if data.Cluster == "" {
		response.FailReturn(c, errcode.InvalidBodyFormat)
		return
	}

	r := rbac.NewDefaultResolver(data.Cluster)

	for _, info := range data.Infos {
		if info.AuthItem == "" {
			continue
		}

		resources, ok := h.cmData[info.AuthItem]
		if !ok {
			res[info.AuthItem] = mapping.Null
			continue
		}

		allowedRead, allowedWrite := true, true
		for _, resource := range strings.Split(resources, ";") {
			allowedRead = isAllowedAccess(r, data.User, resource, info.Namespace, mapping.Read)
		}

		for _, resource := range strings.Split(resources, ";") {
			allowedWrite = isAllowedAccess(r, data.User, resource, info.Namespace, mapping.Write)
		}

		verb := mapping.Null

		if allowedRead && allowedWrite {
			verb = mapping.All
		}
		if allowedWrite {
			verb = mapping.Write
		}
		if allowedRead {
			verb = mapping.Read
		}

		res[info.AuthItem] = verb
	}

	response.SuccessReturn(c, res)
}

// getRolesByUser return roles if namespace specified role and
// clusterRole contained, otherwise just return clusterRole.
// @Summary Get roles
// @Description get roles if namespace specified role and clusterRole contained, otherwise just return clusterRole
// @Tags authorization
// @Param user query string false "user name"
// @Param namespace query string false "namespace name"
// @Param details query string false "details true or false"
// @Success 200 {object} map[string]interface{} "{"clusterRoles":{"total":2,"items":["tenant-admin","reviewer"]},"roles":{"total":0,"items":[]}}"
// @Failure 500 {object} errcode.ErrorInfo
// @Router /api/v1/cube/authorization/roles [get]
func (h *handler) getRolesByUser(c *gin.Context) {
	userName := c.Query("user")
	ns := c.Query("namespace")
	details := c.Query("details")
	byUser := c.Query("byuser")

	if byUser == "true" && userName == "" {
		userName = c.GetString(constants.UserName)
	}

	if userName == "" {
		var r map[string]interface{}
		var err error
		if ns != "" {
			r, err = getRolesByNs(c.Request.Context(), h.Client, ns)
			if err != nil {
				clog.Error(err.Error())
				response.FailReturn(c, errcode.InternalServerError)
				return
			}
		} else {
			r, err = getAllRoles(c.Request.Context(), h.Client)
			if err != nil {
				clog.Error(err.Error())
				response.FailReturn(c, errcode.InternalServerError)
				return
			}
		}
		response.SuccessReturn(c, r)
		return
	}

	user, err := h.GetUser(userName)
	if err != nil {
		clog.Error(err.Error())
		response.FailReturn(c, errcode.InternalServerError)
		return
	}

	roles, clusterRoles, err := h.RolesFor(rbac.User2UserInfo(user.Name), ns)
	if err != nil {
		clog.Error(err.Error())
		response.FailReturn(c, errcode.InternalServerError)
		return
	}

	r := make(map[string]interface{})

	if details == "true" {
		r["roles"] = result{
			Total: len(roles),
			Items: roles,
		}
		r["clusterRoles"] = result{
			Total: len(clusterRoles),
			Items: clusterRoles,
		}
	} else {
		roleNames, clusterRoleNames := makeRoleNamesByPtr(roles, clusterRoles)
		r["roles"] = result{
			Total: len(roleNames),
			Items: roleNames,
		}
		r["clusterRoles"] = result{
			Total: len(clusterRoleNames),
			Items: clusterRoleNames,
		}
	}

	response.SuccessReturn(c, r)
}

// getUsersByRole get all of roles and cluster roles bind to user, with non empty
// namespace will match both Role and ClusterRole, otherwise only clusterRole
// will be matched.
// @Summary Get users
// @Description  get all of roles and cluster roles bind to user, with non empty namespace will match both Role and ClusterRole, otherwise only clusterRole
// @Tags authorization
// @Param role query string false "role name"
// @Param namespace query string false "namespace name"
// @Param details query string false "details true or false"
// @Success 200 {object} result "{"total":1,"items":["admin"]}"
// @Failure 500 {object} errcode.ErrorInfo
// @Router /api/v1/cube/authorization/users [get]
func (h *handler) getUsersByRole(c *gin.Context) {
	roleName := c.Query("role")
	ns := c.Query("namespace")
	details := c.Query("details")

	role := rbacv1.RoleRef{
		Name:     roleName,
		APIGroup: constants.K8sGroupRBAC,
		Kind:     constants.K8sKindClusterRole,
	}

	if len(ns) > 0 {
		role.Kind = constants.K8sKindRole
	}

	users, err := h.UsersFor(role, ns)
	if err != nil {
		clog.Error(err.Error())
		response.FailReturn(c, errcode.InternalServerError)
		return
	}

	r := result{Total: len(users)}

	if details == "true" {
		r.Items = users
	} else {
		r.Items = makeUserNames(users)
	}

	response.SuccessReturn(c, r)
}

// getTenantByUser get all visible tenant for user
// @Summary Get visible tenant
// @Description get all visible tenant for user
// @Tags authorization
// @Param user query string true "user name"
// @Success 200 {object} result "{"total":4,"items":[{"kind":"Tenant","apiVersion":"tenant.kubecube.io/v1","metadata":{"name":"tenant-1","uid":"103a636a-1532-4eb6-a5d1-695fb4007c5a","resourceVersion":"34659","generation":2,"creationTimestamp":"2022-04-28T08:57:33Z","annotations":{"kubecube.io/sync":"1"},"managedFields":[{"manager":"Mozilla","operation":"Update","apiVersion":"tenant.kubecube.io/v1","time":"2022-04-28T08:57:33Z","fieldsType":"FieldsV1","fieldsV1":{"f:spec":{".":{},"f:displayName":{}}}},{"manager":"cube","operation":"Update","apiVersion":"tenant.kubecube.io/v1","time":"2022-04-28T08:57:33Z","fieldsType":"FieldsV1","fieldsV1":{"f:metadata":{"f:annotations":{".":{},"f:kubecube.io/sync":{}}},"f:spec":{"f:namespace":{}},"f:status":{}}}]},"spec":{"displayName":"tenant-1","namespace":"kubecube-tenant-tenant-1"},"status":{}},{"kind":"Tenant","apiVersion":"tenant.kubecube.io/v1","metadata":{"name":"tenant-2","uid":"31de5d32-22f0-445a-9d32-27f87fb82d53","resourceVersion":"24174","generation":2,"creationTimestamp":"2022-04-28T08:17:29Z","annotations":{"kubecube.io/sync":"1"},"managedFields":[{"manager":"Mozilla","operation":"Update","apiVersion":"tenant.kubecube.io/v1","time":"2022-04-28T08:17:29Z","fieldsType":"FieldsV1","fieldsV1":{"f:spec":{".":{},"f:displayName":{}}}},{"manager":"cube","operation":"Update","apiVersion":"tenant.kubecube.io/v1","time":"2022-04-28T08:17:29Z","fieldsType":"FieldsV1","fieldsV1":{"f:metadata":{"f:annotations":{".":{},"f:kubecube.io/sync":{}}},"f:spec":{"f:namespace":{}},"f:status":{}}}]},"spec":{"displayName":"tenant-2","namespace":"kubecube-tenant-tenant-2"},"status":{}},{"kind":"Tenant","apiVersion":"tenant.kubecube.io/v1","metadata":{"name":"tenant-3","uid":"a5756286-bf2b-4094-8c67-c65b4cd2fe7c","resourceVersion":"30156","generation":2,"creationTimestamp":"2022-04-28T08:40:28Z","annotations":{"kubecube.io/sync":"1"},"managedFields":[{"manager":"Mozilla","operation":"Update","apiVersion":"tenant.kubecube.io/v1","time":"2022-04-28T08:40:28Z","fieldsType":"FieldsV1","fieldsV1":{"f:spec":{".":{},"f:displayName":{}}}},{"manager":"cube","operation":"Update","apiVersion":"tenant.kubecube.io/v1","time":"2022-04-28T08:40:28Z","fieldsType":"FieldsV1","fieldsV1":{"f:metadata":{"f:annotations":{".":{},"f:kubecube.io/sync":{}}},"f:spec":{"f:namespace":{}},"f:status":{}}}]},"spec":{"displayName":"tenant-3","namespace":"kubecube-tenant-tenant-3"},"status":{}},{"kind":"Tenant","apiVersion":"tenant.kubecube.io/v1","metadata":{"name":"tenant-4","uid":"0e30568f-1a91-41de-9991-deaa987245eb","resourceVersion":"2936367","generation":2,"creationTimestamp":"2022-05-06T03:35:55Z","annotations":{"kubecube.io/sync":"1"},"managedFields":[{"manager":"Mozilla","operation":"Update","apiVersion":"tenant.kubecube.io/v1","time":"2022-05-06T03:35:55Z","fieldsType":"FieldsV1","fieldsV1":{"f:spec":{".":{},"f:displayName":{}}}},{"manager":"cube","operation":"Update","apiVersion":"tenant.kubecube.io/v1","time":"2022-05-06T03:35:55Z","fieldsType":"FieldsV1","fieldsV1":{"f:metadata":{"f:annotations":{".":{},"f:kubecube.io/sync":{}}},"f:spec":{"f:namespace":{}},"f:status":{}}}]},"spec":{"displayName":"tenant-4","namespace":"kubecube-tenant-tenant-4"},"status":{}}]}"
// @Failure 500 {object} errcode.ErrorInfo
// @Router /api/v1/cube/authorization/tenants [get]
func (h *handler) getTenantByUser(c *gin.Context) {
	user := c.Query("user")
	auth := c.Query("auth")
	ctx := c.Request.Context()
	cli := h.Client

	if len(user) == 0 {
		user = c.GetString(constants.UserName)
	}

	if len(auth) == 0 {
		auth = constants.Readable
	}

	tenants, err := getAccessTenants(h.Interface, user, cli, ctx, auth)
	if err != nil {
		clog.Error(err.Error())
		response.FailReturn(c, errcode.InternalServerError)
		return
	}

	response.SuccessReturn(c, tenants)
}

// getProjectByUser get all visible project for user under tenant
// @Summary Get projects
// @Description get all visible project for user under tenant
// @Tags authorization
// @Param user query string true "user name"
// @Param tenant query string true "tenant name"
// @Success 200 {object} result "{"total":1,"items":[{"kind":"Project","apiVersion":"tenant.kubecube.io/v1","metadata":{"name":"project-1","uid":"bd1d139f-2c22-481b-ad26-a0905eb70651","resourceVersion":"34703","generation":2,"creationTimestamp":"2022-04-28T08:57:41Z","labels":{"kubecube.io/tenant":"tenant-1"},"annotations":{"kubecube.io/sync":"1"},"managedFields":[{"manager":"Mozilla","operation":"Update","apiVersion":"tenant.kubecube.io/v1","time":"2022-04-28T08:57:41Z","fieldsType":"FieldsV1","fieldsV1":{"f:metadata":{"f:labels":{".":{},"f:kubecube.io/tenant":{}}},"f:spec":{".":{},"f:description":{},"f:displayName":{}}}},{"manager":"cube","operation":"Update","apiVersion":"tenant.kubecube.io/v1","time":"2022-04-28T08:57:41Z","fieldsType":"FieldsV1","fieldsV1":{"f:metadata":{"f:annotations":{".":{},"f:kubecube.io/sync":{}}},"f:spec":{"f:namespace":{}},"f:status":{}}}]},"spec":{"displayName":"project-1","description":"project-1","namespace":"kubecube-project-project-1"},"status":{}}]}"
// @Failure 500 {object} errcode.ErrorInfo
// @Router /api/v1/cube/authorization/projects [get]
func (h *handler) getProjectByUser(c *gin.Context) {
	user := c.Query("user")
	tenantArray := c.Query("tenant")
	auth := c.Query("auth")
	ctx := c.Request.Context()
	cli := h.Client
	tenant := strings.Split(tenantArray, "|")
	if len(tenantArray) == 0 {
		tenant = nil
	}
	if len(user) == 0 {
		user = c.GetString(constants.UserName)
	}

	if len(auth) == 0 {
		// default use readable access
		auth = constants.Readable
	}

	projects, err := getAccessProjects(h.Interface, user, cli, ctx, tenant, auth)
	if err != nil {
		clog.Error(err.Error())
		response.FailReturn(c, errcode.InternalServerError)
		return
	}

	response.SuccessReturn(c, projects)
}

// getIdentity show a user identity of platform, tenant or project
// @Summary Show identity
// @Description show a user identity of platform, tenant or project
// @Tags authorization
// @Param user query string true "user name"
// @Success 200 {object} map[string]bool "{"platformAdmin":true,"tenantAdmin":true,"projectAdmin":true}"
// @Router /api/v1/cube/authorization/identities [get]
func (h *handler) getIdentity(c *gin.Context) {
	user := c.Query("user")

	if len(user) == 0 {
		user = c.GetString(constants.UserName)
	}

	r := make(map[string]bool)

	r["platformAdmin"] = isPlatformAdmin(h.Interface, user)
	r["tenantAdmin"] = isTenantAdmin(h.Interface, h.Client, user)
	r["projectAdmin"] = isProjectAdmin(h.Interface, h.Client, user)

	response.SuccessReturn(c, r)
}

// createBinds create roleBinding and clusterRoleBinding
// @Summary Create roleBinding
// @Description create roleBinding and clusterRoleBinding
// @Tags authorization
// @Param roleBinding body rbacv1.RoleBinding true "roleBinding data"
// @Success 200 {string} string "success"
// @Failure 400 {object} errcode.ErrorInfo
// @Failure 500 {object} errcode.ErrorInfo
// @Router /api/v1/cube/authorization/bindings [post]
func (h *handler) createBinds(c *gin.Context) {
	cli := h.Client
	ctx := c.Request.Context()

	roleBinding := &rbacv1.RoleBinding{}
	err := c.ShouldBindJSON(roleBinding)
	if err != nil {
		response.FailReturn(c, errcode.InvalidBodyFormat)
		return
	}
	if len(roleBinding.Subjects) != 1 {
		response.FailReturn(c, errcode.InvalidBodyFormat)
		return
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: v1.ObjectMeta{
			Annotations: map[string]string{constants.SyncAnnotation: "true"},
			Name:        "gen-" + roleBinding.Name,
		},
		Subjects: []rbacv1.Subject{{
			Kind: "User",
			Name: roleBinding.Subjects[0].Name,
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: constants.K8sGroupRBAC,
			Kind:     constants.K8sKindClusterRole,
		},
	}

	if access := access.AllowAccess(constants.LocalCluster, c.Request, constants.CreateVerb, clusterRoleBinding); !access {
		clog.Debug("permission check fail")
		response.FailReturn(c, errcode.ForbiddenErr)
		return
	}
	if access := access.AllowAccess(constants.LocalCluster, c.Request, constants.CreateVerb, roleBinding); !access {
		clog.Debug("permission check fail")
		response.FailReturn(c, errcode.ForbiddenErr)
		return
	}
	// we should create specified ClusterRoleBinding for different RoleRef
	if roleBinding.RoleRef.Kind == constants.K8sKindClusterRole && roleBinding.RoleRef.Name != constants.ReviewerCluster {
		if _, ok := roleBinding.Labels[constants.TenantLabel]; ok {
			clusterRoleBinding.RoleRef.Name = constants.TenantAdminCluster
		}
		if _, ok := roleBinding.Labels[constants.ProjectLabel]; ok {
			clusterRoleBinding.RoleRef.Name = constants.ProjectAdminCluster
		}
		// platform level ignored
	}

	err = cli.Direct().Create(ctx, roleBinding)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, err.Error()))
			return
		}
		clog.Error(err.Error())
		response.FailReturn(c, errcode.InternalServerError)
		return
	}

	if len(clusterRoleBinding.RoleRef.Name) > 0 {
		err = cli.Direct().Create(ctx, clusterRoleBinding)
		if err != nil {
			if errors.IsAlreadyExists(err) {
				response.FailReturn(c, errcode.CustomReturn(http.StatusBadRequest, err.Error()))
				return
			}
			clog.Error(err.Error())
			response.FailReturn(c, errcode.InternalServerError)
			return
		}
	}

	response.SuccessJsonReturn(c, "success")
}

// deleteBinds delete roleBinding and clusterRoleBinding
// @Summary Delete roleBinding
// @Description delete roleBinding and clusterRoleBinding
// @Tags authorization
// @Param name query string true "roleBinding name"
// @Param namespace query string true "roleBinding namespace"
// @Success 200 {string} string "success"
// @Failure 500 {object} errcode.ErrorInfo
// @Router /api/v1/cube/authorization/bindings [delete]
func (h *handler) deleteBinds(c *gin.Context) {
	cli := h.Client
	ctx := c.Request.Context()

	key := types.NamespacedName{
		Name:      c.Query("name"),
		Namespace: c.Query("namespace"),
	}

	roleBinding := rbacv1.RoleBinding{}
	err := cli.Cache().Get(ctx, key, &roleBinding)
	if err != nil {
		if errors.IsNotFound(err) {
			response.SuccessJsonReturn(c, "resource has been deleted")
			return
		}
		clog.Error(err.Error())
		response.FailReturn(c, errcode.InternalServerError)
		return
	}

	if access := access.AllowAccess(constants.LocalCluster, c.Request, constants.DeleteVerb, &roleBinding); !access {
		clog.Debug("permission check fail")
		response.FailReturn(c, errcode.ForbiddenErr)
		return
	}
	if roleBinding.RoleRef.Kind == constants.K8sKindClusterRole {
		clusterRoleBindingName := "gen-" + roleBinding.Name
		crb := &rbacv1.ClusterRoleBinding{}
		if err := cli.Cache().Get(ctx, types.NamespacedName{Name: clusterRoleBindingName}, crb); err != nil {
			if errors.IsNotFound(err) {
				clog.Warn(err.Error())
			} else {
				clog.Error(err.Error())
				response.FailReturn(c, errcode.InternalServerError)
				return
			}
		} else {
			if access := access.AllowAccess(constants.LocalCluster, c.Request, constants.DeleteVerb, crb); !access {
				clog.Debug("permission check fail")
				response.FailReturn(c, errcode.ForbiddenErr)
				return
			}
		}
		err = cli.ClientSet().RbacV1().ClusterRoleBindings().Delete(ctx, clusterRoleBindingName, v1.DeleteOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				clog.Warn(err.Error())
			} else {
				clog.Error(err.Error())
				response.FailReturn(c, errcode.InternalServerError)
				return
			}
		}
	}

	err = cli.ClientSet().RbacV1().RoleBindings(key.Namespace).Delete(ctx, key.Name, v1.DeleteOptions{})
	if err != nil {
		clog.Error(err.Error())
		response.FailReturn(c, errcode.InternalServerError)
		return
	}

	response.SuccessJsonReturn(c, "success")
}

// getClusterRolesByLevel get clusterRoles by hnc level
// @Summary Get roleBinding
// @Description get clusterRoles by level
// @Tags authorization
// @Param level query string true "hnc level"
// @Success 200 {object} result "{"total":1,"items":[{"kind":"ClusterRole","apiVersion":"rbac.authorization.k8s.io/v1","metadata":{"name":"platform-admin","uid":"87851558-7247-4e17-94fa-bf9ddcb48387","resourceVersion":"793","creationTimestamp":"2022-04-28T06:41:26Z","labels":{"app.kubernetes.io/managed-by":"Helm","kubecube.io/rbac":"true","kubecube.io/role":"platform"},"annotations":{"kubecube.io/sync":"true","meta.helm.sh/release-name":"kubecube","meta.helm.sh/release-namespace":"default"},"managedFields":[{"manager":"clusterrole-aggregation-controller","operation":"Apply","apiVersion":"rbac.authorization.k8s.io/v1","time":"2022-04-28T06:41:26Z","fieldsType":"FieldsV1","fieldsV1":{"f:rules":{}}},{"manager":"Go-http-client","operation":"Update","apiVersion":"rbac.authorization.k8s.io/v1","time":"2022-04-28T06:41:26Z","fieldsType":"FieldsV1","fieldsV1":{"f:aggregationRule":{".":{},"f:clusterRoleSelectors":{}},"f:metadata":{"f:annotations":{".":{},"f:kubecube.io/sync":{},"f:meta.helm.sh/release-name":{},"f:meta.helm.sh/release-namespace":{}},"f:labels":{".":{},"f:app.kubernetes.io/managed-by":{},"f:kubecube.io/rbac":{},"f:kubecube.io/role":{}}}}}]},"rules":[{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["pods"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["pods/attach"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["pods/status"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["pods/execescalate"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["pods/exec"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["pods/portforward"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["pods/proxy"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["pods/log"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["replicationcontrollers"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["replicationcontrollers/scale"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["replicationcontrollers/status"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["persistentvolumeclaims"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["persistentvolumeclaims/status"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["configmaps"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["endpoints"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["endpointslices"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["secrets"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["services"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["services/proxy"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["services/status"]},{"verbs":["impersonate","get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["serviceaccounts"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["daemonsets"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["daemonsets/status"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["deployments"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["deployments/rollback"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["deployments/scale"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["deployments/status"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["statefulsets"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["statefulsets/scale"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["statefulsets/status"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["replicasets"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["replicasets/scale"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["replicasets/status"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["controllerrevisions"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["horizontalpodautoscalers"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["horizontalpodautoscalers/status"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["verticalpodautoscalercheckpoints"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["verticalpodautoscalers"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["cronjobs"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["cronjobs/status"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["jobs"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["jobs/status"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["ingresses"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["ingresses/status"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["networkpolicies"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["ingressclasses"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["poddisruptionbudgets"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["poddisruptionbudgets/status"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["nodes"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["persistentvolumes"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["storageclasses"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["bindings"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["events"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["limitranges"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["resourcequotas"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["resourcequotas/status"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["namespaces"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["namespaces/status"]},{"verbs":["create","delete","deletecollection","get","list","patch","update","watch"],"apiGroups":["*"],"resources":["localsubjectaccessreviews"]},{"verbs":["create","delete","deletecollection","get","list","patch","update","watch"],"apiGroups":["*"],"resources":["selfsubjectaccessreviews"]},{"verbs":["create","delete","deletecollection","get","list","patch","update","watch"],"apiGroups":["*"],"resources":["selfsubjectrulesreviews"]},{"verbs":["create","delete","deletecollection","get","list","patch","update","watch"],"apiGroups":["*"],"resources":["subjectaccessreviews"]},{"verbs":["create","delete","deletecollection","get","list","patch","update","watch"],"apiGroups":["*"],"resources":["rolebindings"]},{"verbs":["create","delete","deletecollection","get","list","patch","update","watch"],"apiGroups":["*"],"resources":["roles"]},{"verbs":["create","delete","deletecollection","get","list","patch","update","watch"],"apiGroups":["*"],"resources":["clusterrolebindings"]},{"verbs":["create","delete","deletecollection","get","list","patch","update","watch"],"apiGroups":["*"],"resources":["clusterroles"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["customresourcedefinitions"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["componentstatuses"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["podtemplates"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["mutatingwebhookconfigurations"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["validatingwebhookconfigurations"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["apiservices"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["tokenreviews"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["certificatesigningrequests"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["leases"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["flowschemas"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["prioritylevelconfigurations"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["runtimeclasses"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["priorityclasses"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["csidrivers"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["csinodes"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["volumeattachments"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["podsecuritypolicies"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["cuberesourcequota"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["cuberesourcequota/finalizers"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["cuberesourcequota/status"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["clusters"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["clusters/finalizers"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["tenants"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["tenants/finalizers"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["tenants/status"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["projects"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["projects/finalizers"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["projects/status"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["users"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["users/finalizers"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["users/status"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["keys"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["hotplugs"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["externalresources"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["dashboards"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["alertmanagerconfigs"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["alertmanagers"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["podmonitors"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["probes"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["prometheuses"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["prometheusrules"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["servicemonitors"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["thanosrulers"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["bgpconfigurations"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["bgppeers"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["blockaffinities"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["clusterinformations"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["felixconfigurations"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["globalnetworkpolicies"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["globalnetworksets"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["hostendpoints"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["ipamblocks"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["ipamconfigs"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["ipamhandles"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["ippools"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["kubecontrollersconfigurations"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["networksets"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["hierarchyconfigurations"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["hncconfigurations"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["subnamespaceanchors"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["catalogsources"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["clusterserviceversions"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["installplans"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["operatorconditions"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["operatorgroups"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["operators"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["subscriptions"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["packagemanifests"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["nodelogconfigs"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["logconfigs"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["cephs"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["nfs"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["ipallocations"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["ipranges"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["podstickyips"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["subnets"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["contactgroups"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["notifypolicies"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["notifytemplates"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["silencerules"]},{"verbs":["get","list","watch","create","delete","deletecollection","patch","update"],"apiGroups":["*"],"resources":["loadbalancers"]}],"aggregationRule":{"clusterRoleSelectors":[{"matchLabels":{"rbac.authorization.k8s.io/aggregate-to-platform-admin":"true"}}]}}]}"
// @Failure 500 {object} errcode.ErrorInfo
// @Router /api/v1/cube/authorization/clusterroles [get]
func (h *handler) getClusterRolesByLevel(c *gin.Context) {
	level := c.Query("level")

	labelSelector, err := labels.Parse(fmt.Sprintf("%v=%v", constants.RoleLabel, level))
	if err != nil {
		response.FailReturn(c, errcode.InternalServerError)
		return
	}

	clusterRoleList := rbacv1.ClusterRoleList{}
	err = h.Client.Cache().List(c.Request.Context(), &clusterRoleList, &client.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		response.FailReturn(c, errcode.CustomReturn(http.StatusInternalServerError, "get clusterRoleBinding from k8s failed: %v", err))
		return
	}

	r := result{
		Total: len(clusterRoleList.Items),
		Items: clusterRoleList.Items,
	}

	response.SuccessReturn(c, r)
}

type attributes struct {
	Cluster         string `json:"cluster,omitempty"`
	User            string `json:"user,omitempty"`
	Verb            string `json:"verb,omitempty"`
	Namespace       string `json:"namespace,omitempty"`
	APIGroup        string `json:"apiGroup,omitempty"`
	APIVersion      string `json:"apiVersion,omitempty"`
	Resource        string `json:"resource,omitempty"`
	Subresource     string `json:"subresource,omitempty"`
	Name            string `json:"name,omitempty"`
	ResourceRequest bool   `json:"resourceRequest,omitempty"`
	Path            string `json:"path,omitempty"`
}

// authorization is out way for authorize by KubeCube
func (h *handler) authorization(c *gin.Context) {
	a := &attributes{}
	err := c.ShouldBindJSON(a)
	if err != nil {
		clog.Error(err.Error())
		response.FailReturn(c, errcode.InvalidBodyFormat)
		return
	}

	record := &authorizer.AttributesRecord{
		User:            &userinfo.DefaultInfo{Name: a.User},
		Verb:            a.Verb,
		Namespace:       a.Namespace,
		APIGroup:        a.APIGroup,
		APIVersion:      a.APIVersion,
		Resource:        a.Resource,
		Subresource:     a.Subresource,
		Name:            a.Name,
		ResourceRequest: a.ResourceRequest,
		Path:            a.Path,
	}

	// do auth access in local cluster by default
	// todo: remove it as soon as other caller complete retrofit
	if len(a.Cluster) == 0 {
		a.Cluster = constants.LocalCluster
	}

	cli := clients.Interface().Kubernetes(a.Cluster)
	if cli == nil {
		response.FailReturn(c, errcode.ClusterNotFoundError(a.Cluster))
		return
	}

	rbacResolver := &rbac.DefaultResolver{Cache: cli.Cache()}
	d, _, err := rbacResolver.Authorize(c.Request.Context(), record)
	if err != nil {
		clog.Error(err.Error())
		response.FailReturn(c, errcode.InternalServerError)
		return
	}

	response.SuccessReturn(c, d == authorizer.DecisionAllow)
}

type resourcesAccessInfos struct {
	Cluster string `json:"cluster"`
	Infos   []struct {
		Resource  string `json:"resource"`
		Operator  string `json:"operator"`
		Namespace string `json:"namespace"`
	} `json:"infos"`
}

// resourcesGate tells if given resources can access
func (h *handler) resourcesGate(c *gin.Context) {
	data := &resourcesAccessInfos{}
	if err := c.ShouldBindJSON(data); err != nil {
		clog.Error(err.Error())
		response.FailReturn(c, errcode.InvalidBodyFormat)
		return
	}

	user := c.GetString(constants.UserName)
	result := make(map[string]bool)

	if data.Cluster == "" {
		response.FailReturn(c, errcode.InvalidBodyFormat)
		return
	}

	r := rbac.NewDefaultResolver(data.Cluster)

	for _, info := range data.Infos {
		if info.Resource == "" || info.Operator == "" {
			continue
		}
		// note:we just sort up auth to write and read, take care of it
		var verb string
		if info.Operator == "write" {
			verb = "create"
		}
		if info.Operator == "read" {
			verb = "get"
		}
		record := &authorizer.AttributesRecord{
			User:            &userinfo.DefaultInfo{Name: user},
			Verb:            verb,
			Namespace:       info.Namespace,
			Resource:        info.Resource,
			ResourceRequest: true,
		}
		d, _, err := r.Authorize(c.Request.Context(), record)
		if err != nil {
			clog.Error(err.Error())
			response.FailReturn(c, errcode.InternalServerError)
			return
		}
		result[info.Resource] = d == authorizer.DecisionAllow
	}

	response.SuccessReturn(c, result)
}
