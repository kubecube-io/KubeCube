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

package reporter

import (
	"context"
	"fmt"
	"time"

	"github.com/kubecube-io/kubecube/pkg/clog"
)

var log clog.CubeLogger

const (
	waitPeriod = 500 * time.Millisecond
)

// Reporter reports local cluster info to pivot cluster scout
type Reporter struct {
	Cluster       string
	PivotCubeHost string
	PeriodSecond  int
	WaitSecond    int
	PivotHealthy  bool
}

func (r *Reporter) Initialize() error {
	log = clog.WithName("reporter")
	return nil
}

// Run of reporter will block the goroutine util received stop signal
func (r *Reporter) Run(stop <-chan struct{}) {
	err := r.waitForReady()
	if err != nil {
		log.Fatal("warden start failed: %v", err)
	}

	r.report(stop)
}

func (r *Reporter) waitForReady() error {
	counts := len(checkFuncs)
	if counts < 1 {
		return fmt.Errorf("less 1 components to check ready")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(r.WaitSecond)*time.Second)
	defer cancel()

	readyzCh := make(chan struct{})

	for _, fn := range checkFuncs {
		go readyzCheck(ctx, readyzCh, fn)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-readyzCh:
			counts--
			if counts < 1 {
				return nil
			}
		}
	}
}
