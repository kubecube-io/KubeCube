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

package authentication

type Config struct {
	JwtConfig
	LdapConfig
	GenericConfig
	GitHubConfig
}

func (c *Config) Validate() []error {
	return nil
}

type JwtConfig struct {
	JwtSecret           string `yaml:"jwtSecret,omitempty"`
	TokenExpireDuration int64  `yaml:"tokenExpireDuration, omitempty"`
	JwtIssuer           string `yaml:"jwtIssuer,omitempty"`
}

type LdapConfig struct {
	LdapObjectClass      string `yaml:"ldapObjectClass,omitempty"`
	LdapLoginNameConfig  string `yaml:"ldapLoginNameConfig,omitempty"`
	LdapObjectCategory   string `yaml:"ldapObjectCategory, omitempty"`
	LdapServer           string `yaml:"ldapServer, omitempty"`
	LdapPort             string `yaml:"ldapPort, omitempty"`
	LdapBaseDN           string `yaml:"ldapBaseDN, omitempty"`
	LdapAdminUserAccount string `yaml:"ldapAdminUserAccount, omitempty"`
	LdapAdminPassword    string `yaml:"ldapAdminPassword, omitempty"`
	LdapIsEnable         bool   `yaml:"ldapIsEnable, omitempty"`
}

type GenericConfig struct {
	GenericAuthIsEnable bool
	URL                 string
	Method              string
	Scheme              string
	InsecureSkipVerify  bool
	TLSCert             string
	TLSKey              string
}

type GitHubConfig struct {
	GitHubIsEnable bool
	ClientID       string
	ClientSecret   string
}
