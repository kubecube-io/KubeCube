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
	"time"
)

type checkFunc func() bool

var checkFuncs []checkFunc

// RegisterCheckFunc should be used to registerIfNeed readyz check func
func RegisterCheckFunc(fn checkFunc) {
	checkFuncs = append(checkFuncs, fn)
}

func readyzCheck(ctx context.Context, ch chan struct{}, checkFn checkFunc) {
	for {
		select {
		case <-time.Tick(waitPeriod):
			if checkFn() {
				ch <- struct{}{}
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
