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

package warden

type Config struct {
	InMemberCluster bool

	// report
	Cluster       string
	PivotCubeHost string
	PeriodSecond  int
	WaitSecond    int
	RetryCounts   int

	// sync manager
	PivotClusterKubeConfig string

	// api server
	JwtSecret string
	Addr      string
	Port      int
	TlsCert   string
	TlsKey    string

	// local manager
	AllowPrivileged   bool
	LeaderElect       bool
	WebhookCert       string
	WebhookServerPort int

	// local namespace
	Namespace string
	// feature config map
	FeatureConfigMap string
}

func (c *Config) Validate() []error {
	return nil
}
