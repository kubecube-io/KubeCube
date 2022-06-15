package flags

import "github.com/urfave/cli/v2"

func init() {
	Flags = append(Flags, []cli.Flag{
		&cli.StringFlag{
			Name:        "ingress-nginx-namespace",
			Value:       "ingress-nginx",
			Destination: &CubeOpts.ServiceExtnedOpts.Namespace,
		},
		&cli.StringFlag{
			Name:        "ingress-nginx-tcp-configmap",
			Value:       "tcp-services",
			Destination: &CubeOpts.ServiceExtnedOpts.TcpServiceConfigMap,
		},
		&cli.StringFlag{
			Name:        "ingress-nginx-udp-configmap",
			Value:       "udp-services",
			Destination: &CubeOpts.ServiceExtnedOpts.UdpServiceConfigMap,
		},
	}...)
}
