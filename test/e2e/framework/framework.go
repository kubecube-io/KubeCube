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

package framework

import (
	"github.com/spf13/viper"
)

// Framework supports common operations used by e2e tests; it will keep a client & a namespace for you.
type Framework struct {
	BaseName string

	// Http client
	Host       string
	HttpHelper *HttpHelper

	// Timeouts contains the custom timeouts used during the test execution.
	Timeouts *TimeoutContext

	// preset tenant/project/namespace
	TenantName  string
	ProjectName string
	Namespace   string
}

// NewFramework creates a test framework and sets up a BeforeEach/AfterEach for
// you (you can write additional before/after each functions).
func NewDefaultFramework(baseName string) *Framework {
	return NewFramework(baseName)
}

func NewFramework(baseName string) *Framework {
	f := &Framework{
		BaseName: baseName,
		Timeouts: NewTimeoutContextWithDefaults(),
	}

	// Creating a http client
	f.HttpHelper = NewSingleHttpHelper()
	// preset tenant/project/namspace
	f.TenantName = viper.GetString("kubecube.tenant")
	f.ProjectName = viper.GetString("kubecube.project")
	f.Namespace = viper.GetString("kubecube.namespace")
	return f
}
