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
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	clusterv1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
	hotplugv1 "github.com/kubecube-io/kubecube/pkg/apis/hotplug/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
)

var (
	hotplugLog    clog.CubeLogger
	hotplugClient client.Client
)

type HotplugValidator struct {
	hotplugv1.Hotplug
}

func NewHotplugValidator(mgrClient client.Client) *HotplugValidator {
	hotplugLog = clog.WithName("Webhook").WithName("HotplugValidator")
	hotplugClient = mgrClient
	return &HotplugValidator{}
}

func (t *HotplugValidator) GetObjectKind() schema.ObjectKind {

	return t
}

func (t *HotplugValidator) DeepCopyObject() runtime.Object {
	return &HotplugValidator{}
}

func (t *HotplugValidator) ValidateCreate() (warnings admission.Warnings, err error) {
	log := hotplugLog.WithValues("Validate", t.Name)

	// the cluster exist
	if t.Name != "common" {
		ctx := context.Background()
		var cluster clusterv1.Cluster
		err := hotplugClient.Get(ctx, types.NamespacedName{Name: t.Name}, &cluster)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("the %s not exist", t.Name)
			}
			log.Info("can not get cluster info from pivot cluster, %v", err)
			return nil, fmt.Errorf("the warden server error, %v", err)
		}
	}
	// the component no dump
	m := make(map[string]struct{})
	isRepeat := false
	for _, c := range t.Spec.Component {
		if _, ok := m[c.Name]; ok {
			isRepeat = true
			break
		}
		m[c.Name] = struct{}{}
	}
	if isRepeat {
		return nil, fmt.Errorf("the component name is repeat")
	}
	return nil, nil
}

func (t *HotplugValidator) ValidateUpdate(old runtime.Object) (warnings admission.Warnings, err error) {

	return t.ValidateCreate()
}

func (t *HotplugValidator) ValidateDelete() (warnings admission.Warnings, err error) {

	return nil, nil
}
