package resourcemanage

import "github.com/kubecube-io/kubecube/pkg/utils/filter"

type ExtendParams struct {
	Cluster                  string
	Namespace                string
	ResourceName             string
	Filter                   filter.Filter
	Action                   string
	Username                 string
	NginxNamespace           string
	NginxTcpServiceConfigMap string
	NginxUdpServiceConfigMap string
	Body                     []byte
}
