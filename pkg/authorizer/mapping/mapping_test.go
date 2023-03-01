package mapping

import (
	"reflect"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
)

var cmData = map[string]string{
	"deployment.manage": "deployments;pods;pods/log",
	"services.manage":   "services;endpoints;pods/log",
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
						Resources: []string{"deployments", "pods", "pods/log"},
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
							"pods/log":    Read,
						},
					},
					"services.manage": {
						Verb: Read,
						Resources: map[string]VerbRepresent{
							"services":  Read,
							"endpoints": Read,
							"pods/log":  Read,
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
						Resources: []string{"deployments", "pods", "pods/log"},
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
							"pods/log":    Both,
						},
					},
					"services.manage": {
						Verb: Both,
						Resources: map[string]VerbRepresent{
							"services":  Both,
							"endpoints": Both,
							"pods/log":  Both,
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
						Resources: []string{"services", "pods/log"},
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
							"pods/log":    Both,
						},
					},
					"services.manage": {
						Verb: Write,
						Resources: map[string]VerbRepresent{
							"services":  Both,
							"endpoints": Write,
							"pods/log":  Both,
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
						Resources: []string{"deployments", "pods", "pods/log"},
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

//func TestRoleAuthMapping(t *testing.T) {
//	tests := []struct {
//		name      string
//		roleAuths *RoleAuthBody
//		want      *rbacv1.ClusterRole
//	}{
//		{
//			name: "normal",
//			roleAuths: &RoleAuthBody{
//				AuthItems: map[string]AuthItem{
//					"cxk.manage": {Verb: Read},
//					"deployment.manage": {Verb: Write},
//				},
//			},
//			want: &rbacv1.ClusterRole{
//				Rules: []rbacv1.PolicyRule{
//					{
//						APIGroups: []string{"*"},
//						Resources: []string{"deployments"},
//						Verbs:     writeVerbs.List(),
//					},
//					{
//						APIGroups: []string{"*"},
//						Resources: []string{"pods"},
//						Verbs:     writeVerbs.List(),
//					},
//					{
//						APIGroups: []string{"*"},
//						Resources: []string{"pods/log"},
//						Verbs:     bothVerbs.List(),
//					},
//					{
//						APIGroups: []string{"*"},
//						Resources: []string{"services"},
//						Verbs:     bothVerbs.List(),
//					},
//					{
//						APIGroups: []string{"*"},
//						Resources: []string{"endpoints"},
//						Verbs:     bothVerbs.List(),
//					},
//					{
//						APIGroups: []string{"*"},
//						Resources: []string{"sing"},
//						Verbs:     readVerbs.List(),
//					},
//					{
//						APIGroups: []string{"*"},
//						Resources: []string{"jump"},
//						Verbs:     readVerbs.List(),
//					},
//					{
//						APIGroups: []string{"*"},
//						Resources: []string{"rap"},
//						Verbs:     readVerbs.List(),
//					},
//				},
//			},
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			if got := RoleAuthMapping(tt.roleAuths, cmData); !reflect.DeepEqual(got.Rules, tt.want.Rules) {
//				t.Errorf("RoleAuthMapping() = %v, want %v", got.Rules, tt.want.Rules)
//			}
//		})
//	}
//}
