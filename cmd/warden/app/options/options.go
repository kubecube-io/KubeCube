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

package options

import (
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/warden"
)

type WardenOptions struct {
	GenericWardenOpts *warden.Config
	CubeLoggerOpts    *clog.Config
}

func NewWardenOptions() *WardenOptions {
	wardenOpts := &WardenOptions{
		GenericWardenOpts: &warden.Config{},
		CubeLoggerOpts:    &clog.Config{},
	}

	return wardenOpts
}

func (s *WardenOptions) Validate() []error {
	var errs []error

	errs = append(errs, s.GenericWardenOpts.Validate()...)

	return errs
}

func (s *WardenOptions) NewWarden() *warden.Warden {
	return &warden.Warden{}
}
