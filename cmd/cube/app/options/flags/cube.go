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

// Generic cube flags
func init() {
	Flags = append(Flags, []cli.Flag{
		&cli.BoolFlag{
			Name:        "enable-pprof",
			Value:       false,
			Destination: &CubeOpts.GenericCubeOpts.EnablePprof,
		},
		&cli.StringFlag{
			Name:        "pprof-addr",
			Destination: &CubeOpts.GenericCubeOpts.PprofAddr,
		},
		&cli.StringFlag{
			Name:        "klog-level",
			Value:       "3",
			Destination: &CubeOpts.GenericCubeOpts.KlogLevel,
		},
	}...)
}
