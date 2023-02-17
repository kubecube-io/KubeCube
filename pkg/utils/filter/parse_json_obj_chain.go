/*
Copyright 2022 KubeCube Authors

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

package filter

import (
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var decoder runtime.Decoder

func SetDecoder(scheme *runtime.Scheme) {
	once := sync.Once{}
	once.Do(func() {
		codecFactory := serializer.NewCodecFactory(scheme)
		decoder = codecFactory.UniversalDecoder()
	})
}

func ParseJsonDataHandler(data []byte) (unstructuredObj *unstructured.Unstructured, err error) {
	object := unstructured.Unstructured{}
	_, _, err = decoder.Decode(data, nil, &object)
	if err != nil {
		return nil, fmt.Errorf("can not parser data to internalObject cause: %s ", err.Error())
	}
	return &object, nil
}
