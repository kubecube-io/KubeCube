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

package flags

import (
	"github.com/kubecube-io/kubecube/pkg/authentication/authenticators/jwt"
	"github.com/kubecube-io/kubecube/pkg/authentication/identityprovider/generic"
	"github.com/kubecube-io/kubecube/pkg/authentication/identityprovider/ldap"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/urfave/cli/v2"
)

func init() {
	Flags = append(Flags, []cli.Flag{
		// Ldap Client
		&cli.BoolFlag{
			Name:        "ldap-is-enable",
			Value:       false,
			Destination: &ldap.Config.LdapIsEnable,
		},
		&cli.StringFlag{
			Name:        "ldap-object-class",
			Value:       "person",
			Destination: &ldap.Config.LdapObjectClass,
		},
		&cli.StringFlag{
			Name:        "ldap-login-name-config",
			Value:       "uid",
			Destination: &ldap.Config.LdapLoginNameConfig,
		},
		&cli.StringFlag{
			Name:        "ldap-object-category",
			Destination: &ldap.Config.LdapObjectCategory,
		},
		&cli.StringFlag{
			Name:        "ldap-server",
			Destination: &ldap.Config.LdapServer,
		},
		&cli.StringFlag{
			Name:        "ldap-port",
			Value:       "389",
			Destination: &ldap.Config.LdapPort,
		},
		&cli.StringFlag{
			Name:        "ldap-base",
			Destination: &ldap.Config.LdapBaseDN,
		},
		&cli.StringFlag{
			Name:        "ldap-admin-user-account",
			Destination: &ldap.Config.LdapAdminUserAccount,
		},
		&cli.StringFlag{
			Name:        "ldap-admin-password",
			Destination: &ldap.Config.LdapAdminPassword,
		},

		// jwt
		&cli.Int64Flag{
			Name:        "token-expire-duration",
			Value:       constants.DefaultTokenExpireDuration,
			Destination: &jwt.Config.TokenExpireDuration,
		},
		&cli.StringFlag{
			Name:        "jwt-issuer",
			Destination: &jwt.Config.JwtIssuer,
		},

		// generic
		&cli.BoolFlag{
			Name:        "generic-auth-is-enable",
			Value:       false,
			Destination: &generic.Config.GenericAuthIsEnable,
		},
		&cli.StringFlag{
			Name:        "generic-auth-url",
			Value:       "http://127.0.0.1:8888/api/v1/cube-ultimate/auth",
			Destination: &generic.Config.URL,
		},
		&cli.StringFlag{
			Name:        "generic-auth-method",
			Destination: &generic.Config.Method,
		},
		&cli.StringFlag{
			Name:        "generic-auth-scheme",
			Destination: &generic.Config.Scheme,
		},
		&cli.BoolFlag{
			Name:        "generic-auth-insecure-skip-verify",
			Destination: &generic.Config.InsecureSkipVerify,
		},
		&cli.StringFlag{
			Name:        "generic-auth-tls-cert",
			Destination: &generic.Config.TLSCert,
		},
		&cli.StringFlag{
			Name:        "generic-auth-tls-key",
			Destination: &generic.Config.TLSKey,
		},
	}...)
}
