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
	"net/http"
	"os"
	"time"

	"github.com/kubecube-io/kubecube/pkg/clog"
	multiclient "github.com/kubecube-io/kubecube/pkg/multicluster/client"
	"github.com/kubecube-io/kubecube/pkg/utils/ctls"
)

var log clog.CubeLogger

const (
	// waitPeriod default wait timeout
	waitPeriod = 500 * time.Millisecond
)

// Reporter reports local cluster info to pivot cluster scout
type Reporter struct {
	// Cluster current cluster name
	Cluster string

	// IsMemberCluster indicate if current cluster is member cluster
	IsMemberCluster bool

	// IsWritable indicate if current cluster is writable
	IsWritable bool

	// PivotCubeHost the target warden to reporting
	PivotCubeHost string

	// PeriodSecond is interval time to reporting info
	PeriodSecond int

	// WaitSecond is readyz wait timeout
	WaitSecond int

	// LocalClusterKubeConfig is used for register cluster
	LocalClusterKubeConfig string

	// PivotClient used to connect to pivot cluster k8s-apiserver
	PivotClient multiclient.Client

	// rawLocalKubeConfig is load from LocalClusterKubeConfig
	rawLocalKubeConfig []byte

	// pivotHealthy the pivot cluster healthy status
	pivotHealthy bool

	// http.Client used to reporting heartbeat
	*http.Client
}

func (r *Reporter) Initialize() error {
	log = clog.WithName("reporter")

	// todo:(vela) support tls
	r.Client = &http.Client{
		Transport: ctls.MakeInsecureTransport(),
		Timeout:   5 * time.Second,
	}

	b, err := os.ReadFile(r.LocalClusterKubeConfig)
	if err != nil {
		return err
	}

	r.rawLocalKubeConfig = b

	return nil
}

// Run of reporter will block the goroutine util received stop signal
func (r *Reporter) Run(stop <-chan struct{}) {
	err := r.waitForReady()
	if err != nil {
		log.Fatal("warden start failed: %v", err)
	}

	log.Info("all components ready, try to connect with pivot cluster")

	err = r.registerIfNeed(context.Background())
	if err != nil {
		log.Fatal("warden registerIfNeed failed: %v", err)
	}
	log.Info("ensure cluster %v in control plane success", r.Cluster)

	r.reporting(stop)
}

// waitForReady wait all components of warden ready
func (r *Reporter) waitForReady() error {
	counts := len(checkFuncs)
	if counts < 1 {
		return fmt.Errorf("less 1 components to check ready")
	}

	// wait all components ready in specified time
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
