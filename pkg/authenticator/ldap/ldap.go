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

package ldap

import (
	"fmt"

	"github.com/go-ldap/ldap"
	"github.com/kubecube-io/kubecube/pkg/authenticator"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
)

const (
	ldapAttributeObjectClass    = "objectClass"
	ldapAttributeObjectCategory = "objectCategory"
)

var Config = authenticator.LdapConfig{}

func IsLdapOpen() bool {
	return Config.LdapIsEnable
}

func Authenticate(name string, password string) *errcode.ErrorInfo {

	// create connection by admin account and password
	conn, respInfo := newConn()
	if respInfo != nil {
		return respInfo
	}

	// request to ldap server with user name
	filter := "("
	if Config.LdapObjectCategory != "" || Config.LdapObjectClass != "" {
		filter += "&"
	}
	if Config.LdapObjectCategory != "" {
		filter = fmt.Sprintf("(%s=%s)", ldapAttributeObjectCategory, Config.LdapObjectCategory)
	}
	if Config.LdapObjectClass != "" {
		filter += fmt.Sprintf("(%s=%s)", ldapAttributeObjectClass, Config.LdapObjectClass)
	}
	filter += fmt.Sprintf("(%s=%s))", Config.LdapLoginNameConfig, name)
	result, err := conn.Search(&ldap.SearchRequest{
		BaseDN:       Config.LdapBaseDN,
		Scope:        ldap.ScopeWholeSubtree,
		DerefAliases: ldap.NeverDerefAliases,
		SizeLimit:    1,
		TimeLimit:    0,
		TypesOnly:    false,
		Filter:       filter,
	})
	if err != nil {
		clog.Error("search ldap err: %s", err)
		return errcode.InternalServerError
	}

	// if response result num != 1, password is wrong
	if result == nil || len(result.Entries) != 1 {
		clog.Debug("result is null or result is not only")
		return errcode.AuthenticateError
	}

	// request to ldap server with result username and user password input
	entry := result.Entries[0]
	if err = conn.Bind(entry.DN, password); err != nil {
		clog.Info("verify user password")
		if ldap.IsErrorWithCode(err, ldap.LDAPResultInvalidCredentials) {
			clog.Info("password wrong")
			return errcode.AuthenticateError
		}
		return errcode.InternalServerError
	}

	defer conn.Close()
	return nil
}

func newConn() (ldap.Client, *errcode.ErrorInfo) {
	var host = Config.LdapServer
	if Config.LdapPort != "" {
		host += ":" + Config.LdapPort
	}
	conn, err := ldap.Dial("tcp", host)
	if err != nil {
		clog.Error("connect to ldap server err: %s", err)
		return nil, errcode.LdapConnectError
	}

	err = conn.Bind(Config.LdapAdminUserAccount, Config.LdapAdminPassword)
	if err != nil {
		clog.Error("bind ldap server by admin password error: %s", err)
		return nil, errcode.LdapConnectError
	}
	return conn, nil
}
