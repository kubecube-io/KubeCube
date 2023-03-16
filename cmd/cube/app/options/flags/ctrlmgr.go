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

package flags

import "github.com/urfave/cli/v2"

// controller manager flags
func init() {
	Flags = append(Flags, []cli.Flag{
		&cli.BoolFlag{
			Name:        "leader-elect",
			Value:       false,
			Destination: &CubeOpts.CtrlMgrOpts.LeaderElect,
		},
		&cli.StringFlag{
			Name:        "webhook-cert",
			Destination: &CubeOpts.CtrlMgrOpts.WebhookCert,
		},
		&cli.IntFlag{
			Name:        "webhook-server-port",
			Destination: &CubeOpts.CtrlMgrOpts.WebhookServerPort,
		},
		&cli.BoolFlag{
			Name:        "allow-privileged",
			Destination: &CubeOpts.CtrlMgrOpts.AllowPrivileged,
		},
		&cli.StringFlag{
			Name:        "enable-controllers",
			Value:       "*",
			Destination: &CubeOpts.CtrlMgrOpts.EnableControllers,
		},
		&cli.IntFlag{
			Name:        "scout-wait-timeout-seconds",
			Destination: &CubeOpts.CtrlMgrOpts.ScoutWaitTimeoutSeconds,
			Value:       20,
			Usage:       "timeout wait for warden report heartbeat",
		},
		&cli.IntFlag{
			Name:        "scout-initial-delay-seconds",
			Destination: &CubeOpts.CtrlMgrOpts.ScoutInitialDelaySeconds,
			Value:       10,
			Usage:       "the time that wait for warden start",
		},
	}...)
}
