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

package quota

import (
	quotav1 "github.com/kubecube-io/kubecube/pkg/apis/quota/v1"
)

type Interface interface {
	// Parent get parent quota of current quota return nil
	// if its orphan
	Parent() (*quotav1.CubeResourceQuota, error)

	// Overload return true and reason if this quota exceed limit
	Overload() (bool, string, error)

	// UpdateParentStatus will update parent used resource
	// according to this resource quota. This operation must
	// be idempotent.
	UpdateParentStatus(flush bool) error
}

// todo: to make cube resource quota and resource quota to one interface
