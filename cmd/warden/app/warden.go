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

package app

import (
	"github.com/kubecube-io/kubecube/cmd/warden/app/options"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/warden"

	"github.com/urfave/cli/v2"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"k8s.io/sample-controller/pkg/signals"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	Start = func(c *cli.Context) error {
		if errs := options.WardenOpts.Validate(); len(errs) > 0 {
			return utilerrors.NewAggregate(errs)
		}

		run(options.WardenOpts, signals.SetupSignalHandler())

		return nil
	}
)

func run(s *options.WardenOptions, stop <-chan struct{}) {
	// init cube logger first
	clog.InitCubeLoggerWithOpts(s.CubeLoggerOpts)

	// init setting klog level
	var klogLevel klog.Level
	if err := klogLevel.Set(s.GenericWardenOpts.KlogLevel); err != nil {
		clog.Fatal("klog level set failed: %v", err)
	}

	log.SetLogger(klog.NewKlogr())
	w := warden.NewWardenWithOpts(s.GenericWardenOpts)

	err := w.Initialize()
	if err != nil {
		clog.Fatal("warden initialized failed: %v", err)
	}

	w.Run(stop)
}
