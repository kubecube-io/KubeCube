package config

type NginxConfigMapConfig struct {
	Namespace           string
	TcpServiceConfigMap string
	UdpServiceConfigMap string
}
