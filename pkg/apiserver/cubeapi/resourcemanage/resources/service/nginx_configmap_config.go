package service

type NginxConfigMapConfig struct {
	Namespace           string
	TcpServiceConfigMap string
	UdpServiceConfigMap string
}
