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

package hotplug

import (
	"fmt"

	hotplugv1 "github.com/kubecube-io/kubecube/pkg/apis/hotplug/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type HotplugValidator struct {
	hotplugv1.Hotplug
	isMemberCluster bool
}

func NewHotplugValidator(isMemberCluster bool) *HotplugValidator {
	return &HotplugValidator{isMemberCluster: isMemberCluster}
}

func (t *HotplugValidator) GetObjectKind() schema.ObjectKind {

	return t.GetObjectKind()
}

func (t *HotplugValidator) DeepCopyObject() runtime.Object {
	return &HotplugValidator{}
}

func (t *HotplugValidator) ValidateCreate() error {

	if t.Annotations != nil {
		if v, ok := t.Annotations["kubecube.io/sync"]; ok {
			if v == "true" {
				return nil
			}
		}
	}
	// member cluster do not allow change the config
	if t.isMemberCluster {
		return fmt.Errorf("there is not allow change hotplug config in the member cluster, please do it in the pivot cluster")
	}
	return nil
}

func (t *HotplugValidator) ValidateUpdate(old runtime.Object) error {

	return t.ValidateCreate()
}

func (t *HotplugValidator) ValidateDelete() error {
	// member cluster do not allow change the config
	if t.isMemberCluster {
		return fmt.Errorf("there is not allow change hotplug config in the member cluster, please do it in the pivot cluster")
	}
	return nil
}
