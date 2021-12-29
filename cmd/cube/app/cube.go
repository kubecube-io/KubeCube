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
	"github.com/kubecube-io/kubecube/cmd/cube/app/options"
	"github.com/kubecube-io/kubecube/cmd/cube/app/options/flags"
	"github.com/kubecube-io/kubecube/pkg/apiserver"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/ctrlmgr"
	"github.com/kubecube-io/kubecube/pkg/cube"
	"github.com/kubecube-io/kubecube/pkg/utils/international"
	"github.com/urfave/cli/v2"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/sample-controller/pkg/signals"
)

var (
	Before = func(c *cli.Context) error {
		if !c.Bool("useConfigFile") {
			return nil
		}

		var err error
		flags.CubeOpts, err = options.LoadConfigFromDisk()
		if err != nil {
			return err
		}

		return nil
	}

	Start = func(c *cli.Context) error {
		if errs := flags.CubeOpts.Validate(); len(errs) > 0 {
			return utilerrors.NewAggregate(errs)
		}

		run(flags.CubeOpts, signals.SetupSignalHandler())

		return nil
	}
)

func run(s *options.CubeOptions, stop <-chan struct{}) {
	// init cube logger first
	clog.InitCubeLoggerWithOpts(flags.CubeOpts.CubeLoggerOpts)

	// initialize cube client set
	clients.InitCubeClientSetWithOpts(s.ClientMgrOpts)

	// initialize language managers
	m, err := international.InitGi18nManagers()
	if err != nil {
		clog.Fatal("cube initialized gi18n managers failed: %v", err)
	}
	s.APIServerOpts.Gi18nManagers = m

	c := cube.New(s.GenericCubeOpts)
	c.IntegrateWith("cube-controller-manager", ctrlmgr.NewCtrlMgrWithOpts(s.CtrlMgrOpts))
	c.IntegrateWith("cube-apiserver", apiserver.NewAPIServerWithOpts(s.APIServerOpts))

	err = c.Initialize()
	if err != nil {
		clog.Fatal("cube initialized failed: %v", err)
	}

	c.Run(stop)
}
