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

package jwt

import (
	"testing"

	"k8s.io/api/authentication/v1beta1"
)

func TestGenerateToken(t *testing.T) {

	user1 := &v1beta1.UserInfo{Username: "test"}
	token, err := GetAuthJwtImpl().GenerateToken(user1)
	if err != nil {
		t.Fatal(err)
	}
	userInfo, err := GetAuthJwtImpl().Authentication(token)
	if err != nil {
		t.Fatal(err)

	}
	if userInfo.Username != "test" {
		t.Fail()
	}

}

func TestRefreshToken(t *testing.T) {

	user1 := &v1beta1.UserInfo{Username: "test"}
	token, err := GetAuthJwtImpl().GenerateToken(user1)
	if err != nil {
		t.Fatal(err)
	}

	_, newToken, err := GetAuthJwtImpl().RefreshToken(token)
	if err != nil {
		t.Fatal(err)
	}
	userInfo, err := GetAuthJwtImpl().Authentication(newToken)
	if err != nil {
		t.Fatal(err)
	}
	if userInfo.Username != "test" {
		t.Fail()
	}

}
