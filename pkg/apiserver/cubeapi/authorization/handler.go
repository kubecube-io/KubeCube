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
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	userinfo "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubecube-io/kubecube/pkg/authorizer/rbac"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"
	mgrclient "github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
	"github.com/kubecube-io/kubecube/pkg/utils/response"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
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
}

type result struct {
	Total int         `json:"total"`
	Items interface{} `json:"items"`
}

type handler struct {
	rbac.Interface
	mgrclient.Client
}

func NewHandler() *handler {
	h := new(handler)
	h.Interface = rbac.NewDefaultResolver(constants.LocalCluster)
	h.Client = clients.Interface().Kubernetes(constants.LocalCluster)
	return h
}

// getRolesByUser return roles if namespace specified role and
// clusterRole contained, otherwise just return clusterRole.
// @Summary Get roles
// @Description get roles if namespace specified role and clusterRole contained, otherwise just return clusterRole
// @Tags authorization
// @Param user query string false "user name"
// @Param namespace query string false "namespace name"
// @Param details query string false "details true or false"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} errcode.ErrorInfo
// @Router /api/v1/cube/authorization/roles [get]
func (h *handler) getRolesByUser(c *gin.Context) {
	userName := c.Query("user")
	ns := c.Query("namespace")
	details := c.Query("details")

	if userName == "" {
		r := make(map[string]interface{})
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
// @Success 200 {object} result
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
// @Success 200 {object} result
// @Failure 500 {object} errcode.ErrorInfo
// @Router /api/v1/cube/authorization/tenants [get]
func (h *handler) getTenantByUser(c *gin.Context) {
	user := c.Query("user")
	ctx := c.Request.Context()
	cli := h.Client

	tenants, err := getVisibleTenants(h.Interface, user, cli, ctx)
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
// @Success 200 {object} result
// @Failure 500 {object} errcode.ErrorInfo
// @Router /api/v1/cube/authorization/projects [get]
func (h *handler) getProjectByUser(c *gin.Context) {
	user := c.Query("user")
	tenant := c.Query("tenant")
	ctx := c.Request.Context()
	cli := h.Client

	projects, err := getVisibleProjects(h.Interface, user, cli, ctx, tenant)
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
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cube/authorization/identities [get]
func (h *handler) getIdentity(c *gin.Context) {
	user := c.Query("user")

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

	response.SuccessReturn(c, "success")
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
			response.SuccessReturn(c, "resource has been deleted")
			return
		}
		clog.Error(err.Error())
		response.FailReturn(c, errcode.InternalServerError)
		return
	}

	if roleBinding.RoleRef.Kind == constants.K8sKindClusterRole {
		clusterRoleBindingName := "gen-" + roleBinding.Name
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

	response.SuccessReturn(c, "success")
}

// getClusterRolesByLevel get clusterRoles by hnc level
// @Summary Get roleBinding
// @Description get clusterRoles by level
// @Tags authorization
// @Param level query string true "hnc level"
// @Success 200 {object} result
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

	d, _, err := h.Interface.Authorize(c.Request.Context(), record)
	if err != nil {
		clog.Error(err.Error())
		response.FailReturn(c, errcode.InternalServerError)
		return
	}

	response.SuccessReturn(c, d == authorizer.DecisionAllow)
}
