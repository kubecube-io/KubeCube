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

package md5util

import (
	"crypto/md5"
	"encoding/hex"
)

const (
	salt = "!@#$%^&*()_+"
)

func GetMD5Salt(data string) string {
	m5 := md5.New()
	m5.Write([]byte(data))
	m5.Write([]byte(salt))
	st := m5.Sum(nil)
	return hex.EncodeToString(st)
}
