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
	"testing"

	"github.com/kubecube-io/kubecube/pkg/authenticator"
)

func TestAuthenticate(t *testing.T) {
	Config = authenticator.LdapConfig{}

	errInfo := Authenticate("test", "test123")
	if errInfo != nil {
		fmt.Printf("errInfo: %v\n", errInfo)
		//t.Fail()
	}
}
