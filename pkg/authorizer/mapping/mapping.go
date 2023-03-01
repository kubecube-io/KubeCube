package mapping

import (
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/kubecube-io/kubecube/pkg/clog"
)

const (
	Null  VerbRepresent = "null"
	Read                = "read"
	Write               = "write"
	Both                = "both"
)

type VerbRepresent string

var (
	readVerbs        = sets.NewString("get", "list", "watch")
	writeVerbs       = sets.NewString("create", "delete", "patch", "update", "deletecollection")
	legacyWriteVerbs = sets.NewString("create", "delete", "patch", "update")
	bothVerbs        = readVerbs.Union(writeVerbs)
)

// RoleAuthBody the another transformed form of ClusterRole.
type RoleAuthBody struct {
	ClusterRoleName string              `json:"clusterRoleName,omitempty"`
	AuthItems       map[string]AuthItem `json:"authItems"`
}

type AuthItem struct {
	Verb      VerbRepresent            `json:"verb"`
	Resources map[string]VerbRepresent `json:"resources,omitempty"`
}

// ClusterRoleSplit holds the result of ClusterRole.
type ClusterRoleSplit map[string]VerbRepresent

// SplitClusterRole split ClusterRole as into format:
// deployments: Both
// services: Read
// clusters: Write
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
	if (v1 != v2) && (v1 != Null) && (v2 != Null) {
		return Both
	}
	return Null
}

// verbsAssert asserts verbs as VerbRepresent.
func verbsAssert(verbs []string) VerbRepresent {
	currentVerbs := sets.NewString(verbs...)
	switch {
	case bothVerbs.Equal(currentVerbs):
		return Both
	case currentVerbs.IsSuperset(readVerbs):
		return Read
	case currentVerbs.IsSuperset(writeVerbs):
		return Write
	case currentVerbs.IsSuperset(legacyWriteVerbs):
		return Write
	}
	return Null
}

// ClusterRoleMapping mappings ClusterRole as RoleAuthBody by configmap data.
// cmData format as:
// deployments: "deployments;pods;replicasets;pods/status;deployments/status"
// services: "services;endpoints;pods"
func ClusterRoleMapping(clusterRole *rbacv1.ClusterRole, cmData map[string]string, verbose bool) *RoleAuthBody {
	if len(cmData) == 0 || cmData == nil || clusterRole == nil {
		return nil
	}

	processedClusterRole := SplitClusterRole(clusterRole)

	res := &RoleAuthBody{ClusterRoleName: clusterRole.Name, AuthItems: make(map[string]AuthItem)}
	for k, v := range cmData {
		var (
			visitVerb     VerbRepresent
			interruptVerb VerbRepresent
			placeVerb     VerbRepresent
		)
		authItem := AuthItem{Resources: map[string]VerbRepresent{}}
		resources := strings.Split(v, ";")
		// k: deployment.manager
		// v: deployments;services;pods;pods/logs
		for _, resource := range resources {
			verb, hasRule := processedClusterRole[resource]
			authItem.Resources[resource] = verb

			// if ClusterRole had no this auth item, early set interruptVerb Null.
			if !hasRule || verb == Null {
				interruptVerb = Null
				continue
			}

			// interruptVerb will be Null if meet those condition:
			// 1. Write != Read
			// 3. Null != Read
			// 3. Null != Write
			if (verb != Both) && (visitVerb != Both) && (verb != visitVerb) && (visitVerb != "") {
				interruptVerb = Null
				continue
			}

			// placeVerb will be Null, Write, Read
			if placeVerb == "" || placeVerb == Both {
				placeVerb = verb
			}

			// to visit next resources and verb
			visitVerb = verb
		}

		switch {
		case interruptVerb == Null:
			authItem.Verb = Null
		case visitVerb != placeVerb:
			authItem.Verb = placeVerb
		default:
			authItem.Verb = visitVerb
		}

		if !verbose {
			authItem.Resources = nil
		}

		res.AuthItems[k] = authItem
	}
	return res
}

// RoleAuthMapping mapping RoleAuthBody as ClusterRole by configmap data.
func RoleAuthMapping(roleAuths *RoleAuthBody, cmData map[string]string) *rbacv1.ClusterRole {
	if roleAuths == nil || cmData == nil || len(cmData) == 0 {
		return nil
	}

	rules := make(map[string]VerbRepresent)

	for k, v := range roleAuths.AuthItems {
		if v.Verb == Null {
			continue
		}

		resources, ok := cmData[k]
		if !ok {
			clog.Warn("auth configmap less auth item %v", k)
			continue
		}

		for _, resource := range strings.Split(resources, ";") {
			verb, ok := rules[resource]
			if !ok {
				rules[resource] = v.Verb
				continue
			}
			if verb != v.Verb {
				rules[resource] = Both
			}
		}
	}

	policyRules := make([]rbacv1.PolicyRule, 0, len(rules))

	for resource, verb := range rules {
		verbs := []string{}
		switch verb {
		case Both:
			verbs = bothVerbs.List()
		case Read:
			verbs = readVerbs.List()
		case Write:
			verbs = writeVerbs.List()
		}
		r := rbacv1.PolicyRule{
			APIGroups: []string{"*"},
			Resources: []string{resource},
			Verbs:     verbs,
		}
		policyRules = append(policyRules, r)
	}

	return &rbacv1.ClusterRole{ObjectMeta: v1.ObjectMeta{Name: roleAuths.ClusterRoleName}, Rules: policyRules}
}
