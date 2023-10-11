/*
Copyright 2023 KubeCube Authors

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
