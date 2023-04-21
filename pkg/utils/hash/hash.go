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

package hash

import (
	"fmt"
	"hash"
	"hash/fnv"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/kr/pretty"
)

// DeepHashObject writes specified object to hash using the pretty library
// which follows pointers and prints actual values of the nested objects
// ensuring the hash does not change when a pointer changes.
func DeepHashObject(hasher hash.Hash, objectToWrite interface{}) {
	hasher.Reset()
	pretty.Fprintf(hasher, "%# v", objectToWrite)
}

// GenerateBindingName will generate ClusterRoleBinding name or RoleBinding name.
// sample: {userName}-{hash}
// todo: checkout name
func GenerateBindingName(user, role, namespace string) string {
	var bindingName string
	if len(namespace) == 0 {
		bindingName = user + "-" + role
	} else {
		bindingName = user + "-" + role + "-" + namespace
	}
	hasher := fnv.New32a()
	DeepHashObject(hasher, bindingName)
	return fmt.Sprintf("%s-%s", user, rand.SafeEncodeString(fmt.Sprint(hasher.Sum32())))
}