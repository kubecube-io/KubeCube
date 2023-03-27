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

package ctrlopts

import (
	"strings"

	"github.com/kubecube-io/kubecube/pkg/ctrlmgr/options"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// ControllerInitFns represent controller init func.
type ControllerInitFns map[string]func(manager manager.Manager, opts *options.Options) error

// IsControllerEnabled check if a specified controller enabled or not.
func IsControllerEnabled(name string, controllers []string) bool {
	hasStar := false
	for _, ctrl := range controllers {
		if ctrl == name {
			return true
		}
		if ctrl == "-"+name {
			return false
		}
		if ctrl == "*" {
			hasStar = true
		}
	}
	if !hasStar {
		return false
	}
	return true
}

// ParseControllers parse str into controllers, format as:
// "a1;a2;a3"
// "*"
// "*;-a1"
func ParseControllers(str string) []string {
	if len(str) == 0 {
		return nil
	}
	return strings.Split(str, ";")
}
