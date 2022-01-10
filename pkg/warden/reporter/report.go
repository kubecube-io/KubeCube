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
	"bytes"
	"fmt"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/util/json"

	"github.com/kubecube-io/kubecube/pkg/multicluster/scout"
)

// report do real report loop
func (r *Reporter) report(stop <-chan struct{}) {
	for {
		select {
		case <-time.Tick(time.Duration(r.PeriodSecond) * time.Second):
			if !r.do() {
				r.illPivotCluster()
			} else {
				r.healPivotCluster()
			}
		case <-stop:
			return
		}
	}
}

func (r *Reporter) do() bool {
	w := scout.WardenInfo{Cluster: r.Cluster, ReportTime: time.Now()}

	data, err := json.Marshal(w)
	if err != nil {
		log.Error("json marshal failed: %v", err)
		return false
	}

	reader := bytes.NewReader(data)

	url := fmt.Sprintf("https://%s%s", r.PivotCubeHost, "/api/v1/cube/scout/heartbeat")

	resp, err := r.Client.Post(url, "application/json", reader)
	if err != nil {
		log.Debug("post heartbeat to pivot cluster failed: %v", err)
		return false
	}

	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	return true
}

// illPivotCluster logs when pivot cluster ill
func (r *Reporter) illPivotCluster() {
	if r.PivotHealthy {
		log.Info("disconnect with pivot cluster")
	}
	r.PivotHealthy = false
}

// healPivotCluster logs when reconnected
func (r *Reporter) healPivotCluster() {
	if !r.PivotHealthy {
		log.Info("connected with pivot cluster")
	}
	r.PivotHealthy = true
}
