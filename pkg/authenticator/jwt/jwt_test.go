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

import "testing"

func TestGenerateToken(t *testing.T) {

	var userName = "test"
	token, err := GenerateToken(userName, 0)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseToken(token)
	if err != nil {
		t.Fatal(err)

	}
	if claims.UserInfo.Username != userName {
		t.Fail()
	}

}

func TestRefreshToken(t *testing.T) {

	userName := "test"
	token, err := GenerateToken(userName, 0)
	if err != nil {
		t.Fatal(err)
	}

	newToken, err := RefreshToken(token)
	if err != nil {
		t.Fatal(err)
	}
	newUserName, err := ParseToken(newToken)
	if err != nil {
		t.Fatal(err)
	}
	if newUserName.UserInfo.Username != userName {
		t.Fail()
	}

}
