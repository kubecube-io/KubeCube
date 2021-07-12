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

package cube

import (
	"fmt"

	_ "net/http/pprof"

	"github.com/kubecube-io/kubecube/pkg/clog"

	"net/http"
)

// Runnable run components after initialized
type Runnable interface {
	Run(stop <-chan struct{})
	Initialize() error
}

type Cube struct {
	*Config

	RunnableComponents map[string]Runnable
}

var _ Runnable = &Cube{}

func New(c *Config) *Cube {
	return &Cube{Config: c, RunnableComponents: make(map[string]Runnable)}
}

func (c *Cube) IntegrateWith(name string, r Runnable) {
	if name == "" {
		clog.Fatal("component name is empty")
	}

	if r == nil {
		clog.Fatal("component %s is nil", name)
	}

	if _, dup := c.RunnableComponents[name]; dup {
		clog.Fatal("component %s already set up", name)
	}

	c.RunnableComponents[name] = r
}

func (c *Cube) Run(stop <-chan struct{}) {
	if c.EnablePprof {
		go func() {
			if err := http.ListenAndServe(c.PprofAddr, nil); err != nil {
				clog.Error("unable to start pprof: %v", err.Error())
			}
		}()
	}

	for name, c := range c.RunnableComponents {
		// Run action is non blocking
		go c.Run(stop)
		clog.Info("component %s already started", name)
	}

	<-stop
}

// Initialize init all components of cube, block util initialization
// completed or error occurred.
func (c *Cube) Initialize() error {
	if len(c.RunnableComponents) < 1 {
		clog.Fatal("cube have no components to initialize")
	}

	// initialize runnable components of cube
	for name, c := range c.RunnableComponents {
		if err := c.Initialize(); err != nil {
			return fmt.Errorf("component %s initialize failed: %v", name, err)
		}
		clog.Info("component %s already initialized", name)
	}

	return nil
}
