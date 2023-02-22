package mapping

import (
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	null  VerbRepresent = ""
	read                = "read"
	write               = "write"
	both                = "both"
)

type VerbRepresent string

var (
	readVerbs  = sets.NewString("get", "list", "watch")
	writeVerbs = sets.NewString("create", "delete", "patch", "update", "deletecollection")
	bothVerbs  = readVerbs.Union(writeVerbs)
)

// RoleAuthBody the another transformed form of ClusterRole.
type RoleAuthBody struct {
	AuthItems map[string]AuthItem `json:"authItems"`
}

type AuthItem struct {
	Verb      VerbRepresent `json:"verb"`
	Resources []string      `json:"resources,omitempty"`
}

// ClusterRoleSplit holds the result of ClusterRole.
type ClusterRoleSplit map[string]VerbRepresent

// SplitClusterRole split ClusterRole as into format:
// deployments: both
// services: read
// clusters: write
func SplitClusterRole(clusterRole *rbacv1.ClusterRole) ClusterRoleSplit {
	res := make(map[string]VerbRepresent)
	if clusterRole == nil {
		return res
	}
	for _, rule := range clusterRole.Rules {
		verb := verbsAssert(rule.Verbs)
		for _, resource := range rule.Resources {
			if verbRepresent, ok := res[resource]; ok {
				res[resource] = verbsMerge(verbRepresent, verb)
			} else {
				res[resource] = verb
			}
		}
	}
	return res
}

func verbsMerge(v1, v2 VerbRepresent) VerbRepresent {
	if v1 == v2 {
		return v1
	}
	if (v1 != v2) && (v1 != null) && (v2 != null) {
		return both
	}
	return null
}

// verbsAssert asserts verbs as VerbRepresent.
func verbsAssert(verbs []string) VerbRepresent {
	currentVerbs := sets.NewString(verbs...)
	switch {
	case bothVerbs.Equal(currentVerbs):
		return both
	case readVerbs.Equal(currentVerbs):
		return read
	case writeVerbs.Equal(currentVerbs):
		return write
	}
	return null
}

// ClusterRoleMapping mappings ClusterRole as RoleAuthBody by configmap data.
// cmData format as:
// deployments: "deployments;pods;replicasets;pods/status;deployments/status"
// services: "services;endpoints;pods"
func ClusterRoleMapping(clusterRole *rbacv1.ClusterRole, cmData map[string]string) *RoleAuthBody {
	if len(cmData) == 0 || cmData == nil || clusterRole == nil {
		return nil
	}

	processedClusterRole := SplitClusterRole(clusterRole)

	res := &RoleAuthBody{AuthItems: make(map[string]AuthItem)}
	for k, v := range cmData {
		visitVerb := null
		resources := strings.Split(v, ";")
		for i, resource := range resources {
			verb, ok := processedClusterRole[resource]
			if !ok {
				// if ClusterRole had no this auth item, early out.
				res.AuthItems[k] = AuthItem{Verb: null, Resources: resources}
				goto OUT
			}

			// init first visit verb or expand verb when meet 'both' verb.
			if i == 0 || verb == both {
				visitVerb = verb
			}

		}
	OUT: // out here to visit next
	}
	return nil
}
