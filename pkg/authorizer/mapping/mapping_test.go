package mapping

import (
	"reflect"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
)

var cmData = map[string]string{
	"deployment.manage": "deployments;pods;pods/logs",
	"services.manage":   "services;endpoints;pods/logs",
	"cxk.manage":        "sing;jump;rap",
}

func TestClusterRoleMapping(t *testing.T) {
	tests := []struct {
		name        string
		clusterRole *rbacv1.ClusterRole
		verbose     bool
		want        *RoleAuthBody
	}{
		{
			name:    "all Read",
			verbose: true,
			clusterRole: &rbacv1.ClusterRole{
				Rules: []rbacv1.PolicyRule{
					{
						Resources: []string{"deployments", "pods", "pods/logs"},
						Verbs:     []string{"get", "list", "watch"},
					},
					{
						Resources: []string{"services"},
						Verbs:     []string{"get", "list", "watch"},
					},
					{
						Resources: []string{"endpoints"},
						Verbs:     []string{"get", "list", "watch"},
					},
					{
						Resources: []string{"sing", "jump", "rap"},
						Verbs:     []string{"get", "list", "watch"},
					},
				},
			},
			want: &RoleAuthBody{
				AuthItems: map[string]AuthItem{
					"deployment.manage": {
						Verb: Read,
						Resources: map[string]VerbRepresent{
							"deployments": Read,
							"pods":        Read,
							"pods/logs":   Read,
						},
					},
					"services.manage": {
						Verb: Read,
						Resources: map[string]VerbRepresent{
							"services":  Read,
							"endpoints": Read,
							"pods/logs": Read,
						},
					},
					"cxk.manage": {
						Verb: Read,
						Resources: map[string]VerbRepresent{
							"sing": Read,
							"jump": Read,
							"rap":  Read,
						},
					},
				},
			},
		},
		{
			name:    "all Both",
			verbose: true,
			clusterRole: &rbacv1.ClusterRole{
				Rules: []rbacv1.PolicyRule{
					{
						Resources: []string{"deployments", "pods", "pods/logs"},
						Verbs:     []string{"create", "delete", "patch", "update", "deletecollection", "get", "list", "watch"},
					},
					{
						Resources: []string{"services"},
						Verbs:     []string{"create", "delete", "patch", "update", "deletecollection", "get", "list", "watch"},
					},
					{
						Resources: []string{"endpoints"},
						Verbs:     []string{"create", "delete", "patch", "update", "deletecollection", "get", "list", "watch"},
					},
					{
						Resources: []string{"sing", "jump", "rap"},
						Verbs:     []string{"create", "delete", "patch", "update", "deletecollection", "get", "list", "watch"},
					},
				},
			},
			want: &RoleAuthBody{
				AuthItems: map[string]AuthItem{
					"deployment.manage": {
						Verb: Both,
						Resources: map[string]VerbRepresent{
							"deployments": Both,
							"pods":        Both,
							"pods/logs":   Both,
						},
					},
					"services.manage": {
						Verb: Both,
						Resources: map[string]VerbRepresent{
							"services":  Both,
							"endpoints": Both,
							"pods/logs": Both,
						},
					},
					"cxk.manage": {
						Verb: Both,
						Resources: map[string]VerbRepresent{
							"sing": Both,
							"jump": Both,
							"rap":  Both,
						},
					},
				},
			},
		},
		{
			name:    "complex",
			verbose: true,
			clusterRole: &rbacv1.ClusterRole{
				Rules: []rbacv1.PolicyRule{
					{
						Resources: []string{"deployments"},
						Verbs:     []string{"get", "list", "watch"},
					},
					{
						Resources: []string{"pods"},
						Verbs:     []string{"create", "delete", "patch", "update", "deletecollection"},
					},
					{
						Resources: []string{"services", "pods/logs"},
						Verbs:     []string{"create", "delete", "patch", "update", "deletecollection", "get", "list", "watch"},
					},
					{
						Resources: []string{"endpoints"},
						Verbs:     []string{"create", "delete", "patch", "update", "deletecollection"},
					},
					{
						Resources: []string{"sing", "jump", "rap"},
						Verbs:     []string{"get", "list", "watch", "create", "delete"},
					},
				},
			},
			want: &RoleAuthBody{
				AuthItems: map[string]AuthItem{
					"deployment.manage": {
						Verb: Null,
						Resources: map[string]VerbRepresent{
							"deployments": Read,
							"pods":        Write,
							"pods/logs":   Both,
						},
					},
					"services.manage": {
						Verb: Write,
						Resources: map[string]VerbRepresent{
							"services":  Both,
							"endpoints": Write,
							"pods/logs": Both,
						},
					},
					"cxk.manage": {
						Verb: Read,
						Resources: map[string]VerbRepresent{
							"sing": Read,
							"jump": Read,
							"rap":  Read,
						},
					},
				},
			},
		},
		{
			name:    "all Read",
			verbose: false,
			clusterRole: &rbacv1.ClusterRole{
				Rules: []rbacv1.PolicyRule{
					{
						Resources: []string{"deployments", "pods", "pods/logs"},
						Verbs:     []string{"get", "list", "watch"},
					},
					{
						Resources: []string{"services"},
						Verbs:     []string{"get", "list", "watch"},
					},
					{
						Resources: []string{"endpoints"},
						Verbs:     []string{"get", "list", "watch"},
					},
					{
						Resources: []string{"sing", "jump", "rap"},
						Verbs:     []string{"get", "list", "watch"},
					},
				},
			},
			want: &RoleAuthBody{
				AuthItems: map[string]AuthItem{
					"deployment.manage": {
						Verb: Read,
					},
					"services.manage": {
						Verb: Read,
					},
					"cxk.manage": {
						Verb: Read,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClusterRoleMapping(tt.clusterRole, cmData, tt.verbose); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ClusterRoleMapping() = %v, want %v", got, tt.want)
			}
		})
	}
}
