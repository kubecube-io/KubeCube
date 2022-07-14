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

package options

import (
	"fmt"

	"github.com/spf13/viper"

	"github.com/kubecube-io/kubecube/pkg/apiserver"
	"github.com/kubecube-io/kubecube/pkg/authentication"
	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/ctrlmgr"
	"github.com/kubecube-io/kubecube/pkg/cube"
)

const (
	defaultConfiguration = "kubecube"
	defaultConfigPath    = "/etc/kubecube"
)

type CubeOptions struct {
	GenericCubeOpts *cube.Config
	APIServerOpts   *apiserver.Config
	CtrlMgrOpts     *ctrlmgr.Config
	ClientMgrOpts   *clients.Config
	CubeLoggerOpts  *clog.Config
	AuthMgrOpts     *authentication.Config
}

func NewCubeOptions() *CubeOptions {
	cubeOpts := &CubeOptions{
		GenericCubeOpts: &cube.Config{},
		APIServerOpts:   &apiserver.Config{},
		CtrlMgrOpts:     &ctrlmgr.Config{},
		ClientMgrOpts:   &clients.Config{},
		CubeLoggerOpts:  &clog.Config{},
		AuthMgrOpts:     &authentication.Config{},
	}

	return cubeOpts
}

func LoadConfigFromDisk() (*CubeOptions, error) {
	viper.SetConfigName(defaultConfiguration)
	viper.AddConfigPath(defaultConfigPath)
	viper.AddConfigPath(".")
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil, err
		} else {
			return nil, fmt.Errorf("error parsing configuration file %s", err)
		}
	}

	conf := NewCubeOptions()

	if err := viper.Unmarshal(conf); err != nil {
		return nil, err
	}

	return conf, nil
}

// Validate verify options for every component
// todo(weilaaa): complete it
func (s *CubeOptions) Validate() []error {
	var errs []error

	errs = append(errs, s.APIServerOpts.Validate()...)
	errs = append(errs, s.ClientMgrOpts.Validate()...)
	errs = append(errs, s.CtrlMgrOpts.Validate()...)

	return errs
}

func (s *CubeOptions) NewCube() *cube.Cube {
	return &cube.Cube{}
}
