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
	"fmt"
	"os"

	"github.com/kubecube-io/kubecube/pkg/clients"
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
	// Read config.yaml
	f.ReadEnvConfig()
	// Create strong k8s client
	clients.InitCubeClientSetWithOpts(nil)
	// Creating a http client
	f.HttpHelper = NewHttpHelper().Login()
	// preset tenant/project/namspace
	f.TenantName = viper.GetString("kubecube.tenant")
	f.ProjectName = viper.GetString("kubecube.project")
	f.Namespace = viper.GetString("kubecube.namespace")
	return f
}

// Read env config
func (f *Framework) ReadEnvConfig() {
	// todo commond line
	cfgFile := ""
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		current, err := os.Getwd()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		// 在当前目录下面查找名为 "config.yaml" 的配置文件
		viper.AddConfigPath(current)
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}
	viper.SetEnvPrefix("kubecube")
	// 读取匹配的环境变量
	viper.AutomaticEnv()
	// 如果有配置文件，则读取它
	if err := viper.ReadInConfig(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
