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
	"github.com/urfave/cli/v2"
)

var (
	WardenOpts = NewWardenOptions()

	Flags = []cli.Flag{
		// generic
		&cli.BoolFlag{
			Name:        "in-member-cluster",
			Value:       true,
			Destination: &WardenOpts.GenericWardenOpts.InMemberCluster,
		},
		&cli.BoolFlag{
			Name:        "is-cluster-writable",
			Value:       true,
			Destination: &WardenOpts.GenericWardenOpts.IsWritable,
		},
		&cli.StringFlag{
			Name:        "local-cluster-kubeconfig",
			Destination: &WardenOpts.GenericWardenOpts.LocalClusterKubeConfig,
		},
		&cli.StringFlag{
			Name:        "pivot-cluster-kubeconfig",
			Destination: &WardenOpts.GenericWardenOpts.PivotClusterKubeConfig,
		},
		&cli.StringFlag{
			Name:        "cluster",
			Destination: &WardenOpts.GenericWardenOpts.Cluster,
		},
		&cli.StringFlag{
			Name:        "klog-level",
			Value:       "3",
			Destination: &WardenOpts.GenericWardenOpts.KlogLevel,
		},

		// api server
		&cli.StringFlag{
			Name:        "addr",
			Value:       "0.0.0.0",
			Destination: &WardenOpts.GenericWardenOpts.Addr,
		},
		&cli.IntFlag{
			Name:        "port",
			Value:       7443,
			Destination: &WardenOpts.GenericWardenOpts.Port,
		},
		&cli.StringFlag{
			Name:        "tls-cert",
			Destination: &WardenOpts.GenericWardenOpts.TlsCert,
		},
		&cli.StringFlag{
			Name:        "tls-key",
			Destination: &WardenOpts.GenericWardenOpts.TlsKey,
		},

		// reporter
		&cli.StringFlag{
			Name:        "pivot-cube-host",
			Destination: &WardenOpts.GenericWardenOpts.PivotCubeHost,
		},
		&cli.IntFlag{
			Name:        "period-second",
			Value:       3,
			Destination: &WardenOpts.GenericWardenOpts.PeriodSecond,
		},
		&cli.IntFlag{
			Name:        "wait-second",
			Value:       7,
			Destination: &WardenOpts.GenericWardenOpts.WaitSecond,
		},

		// local manager
		&cli.BoolFlag{
			Name:        "leader-elect",
			Value:       false,
			Destination: &WardenOpts.GenericWardenOpts.LeaderElect,
		},
		&cli.StringFlag{
			Name:        "webhook-cert",
			Value:       "/etc/tls",
			Destination: &WardenOpts.GenericWardenOpts.WebhookCert,
		},
		&cli.IntFlag{
			Name:        "webhook-server-port",
			Value:       8443,
			Destination: &WardenOpts.GenericWardenOpts.WebhookServerPort,
		},
		&cli.BoolFlag{
			Name:        "allow-privileged",
			Value:       true,
			Destination: &WardenOpts.GenericWardenOpts.AllowPrivileged,
		},
		&cli.StringFlag{
			Name:        "enable-controllers",
			Value:       "*",
			Destination: &WardenOpts.GenericWardenOpts.EnableControllers,
		},

		// rotate flags
		&cli.StringFlag{
			Name:        "log-file",
			Value:       "/etc/logs/warden.log",
			Destination: &WardenOpts.CubeLoggerOpts.LogFile,
		},
		&cli.IntFlag{
			Name:        "max-size",
			Value:       1000,
			Destination: &WardenOpts.CubeLoggerOpts.MaxSize,
		},
		&cli.IntFlag{
			Name:        "max-backups",
			Value:       7,
			Destination: &WardenOpts.CubeLoggerOpts.MaxBackups,
		},
		&cli.IntFlag{
			Name:        "max-age",
			Value:       1,
			Destination: &WardenOpts.CubeLoggerOpts.MaxAge,
		},
		&cli.BoolFlag{
			Name:        "compress",
			Value:       true,
			Destination: &WardenOpts.CubeLoggerOpts.Compress,
		},

		// logger flags
		&cli.StringFlag{
			Name:        "log-level",
			Value:       "info",
			Destination: &WardenOpts.CubeLoggerOpts.LogLevel,
		},
		&cli.BoolFlag{
			Name:        "json-encode",
			Value:       false,
			Destination: &WardenOpts.CubeLoggerOpts.JsonEncode,
		},
		&cli.StringFlag{
			Name:        "stacktrace-level",
			Value:       "error",
			Destination: &WardenOpts.CubeLoggerOpts.StacktraceLevel,
		},
		&cli.StringFlag{
			Name:        "ingress-nginx-namespace",
			Value:       "ingress-nginx",
			Destination: &WardenOpts.GenericWardenOpts.NginxNamespace,
		},
		&cli.StringFlag{
			Name:        "ingress-nginx-tcp-configmap",
			Value:       "tcp-services",
			Destination: &WardenOpts.GenericWardenOpts.NginxTcpServiceConfigMap,
		},
		&cli.StringFlag{
			Name:        "ingress-nginx-udp-configmap",
			Value:       "udp-services",
			Destination: &WardenOpts.GenericWardenOpts.NginxUdpServiceConfigMap,
		},
	}
)
