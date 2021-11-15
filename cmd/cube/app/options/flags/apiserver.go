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

// api-server flags
func init() {
	Flags = append(Flags, []cli.Flag{
		// Server flags
		&cli.StringFlag{
			Name:        "bind-addr",
			Value:       "0.0.0.0",
			Destination: &CubeOpts.APIServerOpts.BindAddr,
		},
		&cli.IntFlag{
			Name:        "insecure-port",
			Destination: &CubeOpts.APIServerOpts.InsecurePort,
		},
		&cli.IntFlag{
			Name:        "secure-port",
			Value:       7443,
			Destination: &CubeOpts.APIServerOpts.SecurePort,
		},
		&cli.IntFlag{
			Name:        "generic-port",
			Value:       7777,
			Destination: &CubeOpts.APIServerOpts.GenericPort,
		},
		&cli.StringFlag{
			Name:        "tls-cert",
			Destination: &CubeOpts.APIServerOpts.TlsCert,
		},
		&cli.StringFlag{
			Name:        "tls-key",
			Destination: &CubeOpts.APIServerOpts.TlsKey,
		},
		&cli.StringFlag{
			Name:        "ca-cert",
			Destination: &CubeOpts.APIServerOpts.CaCert,
		},
		&cli.StringFlag{
			Name:        "ca-key",
			Destination: &CubeOpts.APIServerOpts.CaKey,
		},
	}...)
}
