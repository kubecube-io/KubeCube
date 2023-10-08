package config

type Config struct {
	// QPS indicates the maximum QPS to the master from this client.
	// If it's zero, the created RESTClient will use DefaultQPS: 5
	QPS float32 `json:"qps,omitempty"`

	// Maximum burst for throttle.
	// If it's zero, the created RESTClient will use DefaultBurst: 10.
	Burst int `json:"burst,omitempty"`

	// The maximum length of time to wait before giving up on a server request. A value of zero means no timeout, Default: 0.
	TimeoutSecond int `json:"timeoutSecond,omitempty"`

	// the cluster discovery cache sync enable, default is false
	ClusterCacheSyncEnable bool `json:"clusterCacheSyncEnable,omitempty"`

	// the cluster discovery cache sync interval，unit is second, default is 60
	ClusterCacheSyncInterval int `json:"clusterCacheSyncInterval,omitempty"`
}
