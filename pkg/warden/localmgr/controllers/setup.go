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

package controllers

import (
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/warden/localmgr/controllers/hotplug"
	"github.com/kubecube-io/kubecube/pkg/warden/localmgr/controllers/olm"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var setupFns []func(manager manager.Manager) error

func init() {
	// setup controllers
	setupFns = append(setupFns, hotplug.SetupWithManager)
	setupFns = append(setupFns, olm.SetupWithManager)
}

// SetupWithManager set up controllers into manager
func SetupWithManager(m manager.Manager) error {
	for _, f := range setupFns {
		if err := f(m); err != nil {
			if kindMatchErr, ok := err.(*meta.NoKindMatchError); ok {
				clog.Warn("CRD %v is not installed, its controller will dry run!", kindMatchErr.GroupKind)
				continue
			}
			return err
		}
	}
	return nil
}
