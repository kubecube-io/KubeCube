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

package env

import (
	"os"

	"github.com/kubecube-io/kubecube/pkg/utils/constants"
)

func WardenImage() string {
	return os.Getenv("WARDEN_IMAGE")
}

func WardenInitImage() string {
	return os.Getenv("WARDEN_INIT_IMAGE")
}

func PivotCubeHost() string {
	r := os.Getenv("PIVOT_CUBE_HOST")
	if r == "" {
		r = constants.DefaultPivotCubeHost
	}
	return r
}

func PivotCubeClusterIPSvc() string {
	r := os.Getenv("PIVOT_CUBE_CLUSTER_IP_SVC")
	if r == "" {
		r = constants.DefaultPivotCubeClusterIPSvc
	}
	return r
}

func AuditIsEnable() bool {
	r := os.Getenv("AUDIT_IS_ENABLE")
	if r == "false" {
		return false
	}
	return true
}

func AuditSVC() string {
	r := os.Getenv("AUDIT_SVC")
	if r == "" {
		r = constants.DefaultAuditSvc
	}
	return r
}

func JwtSecret() string {
	return os.Getenv("JWT_SECRET")
}

func NodeIP() string {
	return os.Getenv("NODE_IP")
}
