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

package apiserver

import (
	"github.com/kubecube-io/kubecube/pkg/authentication"
)

type Config struct {
	HttpConfig
	authentication.LdapConfig
	authentication.JwtConfig
	authentication.GenericConfig
}

type HttpConfig struct {
	BindAddr     string `yaml:"bindAddr,omitempty"`
	InsecurePort int    `yaml:"insecurePort,omitempty"`
	SecurePort   int    `yaml:"securePort, omitempty"`
	GenericPort  int    `yaml:"genericPort,omitempty"`
	TlsCert      string `yaml:"tlsCert,omitempty"`
	TlsKey       string `yaml:"tlsKey,omitempty"`
	CaCert       string `yaml:"caCert,omitempty"`
	CaKey        string `yaml:"caKey,omitempty"`
}

func (c *Config) Validate() []error {
	return nil
}
