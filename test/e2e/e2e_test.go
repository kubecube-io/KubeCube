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

package e2e

import (
	"math/rand"
	"testing"
	"time"

	// test sources
	"github.com/kubecube-io/kubecube/pkg/clog"
	_ "github.com/kubecube-io/kubecube/test/e2e/k8sproxy"
	_ "github.com/kubecube-io/kubecube/test/e2e/openapi"
	_ "github.com/kubecube-io/kubecube/test/e2e/tenant"
	_ "github.com/kubecube-io/kubecube/test/e2e/user"
	_ "github.com/kubecube-io/kubecube/test/e2e/yamldeploy"
)

// entrance
func TestMain(m *testing.M) {
	clog.Info("e2e begin")
	Init()
	rand.Seed(time.Now().UnixNano())
	m.Run()
	Clean()
	clog.Info("e2e end")
}

// start e2e test
func TestE2E(t *testing.T) {
	RunE2ETests(t)
}
