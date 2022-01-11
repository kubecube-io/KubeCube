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
	"context"
	"fmt"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/wait"

	clusterv1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
	"github.com/kubecube-io/kubecube/pkg/multicluster/scout"
	"github.com/kubecube-io/kubecube/pkg/utils/kubeconfig"
)

// reporting do real report loop
func (r *Reporter) reporting(stop <-chan struct{}) {
	for {
		select {
		case <-time.Tick(time.Duration(r.PeriodSecond) * time.Second):
			healthy := r.report()
			if healthy {
				r.healPivotCluster()
			} else {
				r.illPivotCluster()
			}
		case <-stop:
			return
		}
	}
}

// registerIfNeed register current cluster to pivot cluster if need
func (r *Reporter) registerIfNeed(ctx context.Context) error {
	// todo: remove it when we dont need KubernetesAPIEndpoint anymore
	cfg, err := kubeconfig.LoadKubeConfigFromBytes(r.rawLocalKubeConfig)
	if err != nil {
		return err
	}

	return wait.Poll(3*time.Second, 15*time.Second, func() (done bool, err error) {
		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: r.Cluster},
			Spec: clusterv1.ClusterSpec{
				KubeConfig:            r.rawLocalKubeConfig,
				IsMemberCluster:       r.IsMemberCluster,
				KubernetesAPIEndpoint: cfg.Host,
			},
		}
		err = r.PivotClient.Direct().Create(ctx, cluster)
		if err != nil {
			if errors.IsAlreadyExists(err) {
				log.Debug("cluster cr %v is already exist", cluster.Name)
				return true, nil
			}
			log.Warn("create cluster %v failed: %v", err)
			return false, nil
		}
		return true, nil
	})
}

func (r *Reporter) report() bool {
	w := scout.WardenInfo{Cluster: r.Cluster, ReportTime: time.Now()}

	resp, err := r.do(w)
	if err != nil {
		log.Debug("warden report failed: %v", err)
		return false
	}

	if resp.StatusCode != http.StatusOK {
		return false
	}

	return true
}

func (r *Reporter) do(info scout.WardenInfo) (*http.Response, error) {
	data, err := json.Marshal(info)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(data)

	url := fmt.Sprintf("https://%s%s", r.PivotCubeHost, "/api/v1/cube/scout/heartbeat")

	resp, err := r.Client.Post(url, "application/json", reader)
	if err != nil {
		return nil, err
	}

	_ = resp.Body.Close()

	return resp, nil
}

// illPivotCluster logs when pivot cluster ill
func (r *Reporter) illPivotCluster() {
	if r.pivotHealthy {
		log.Info("disconnect with pivot cluster")
	}
	r.pivotHealthy = false
}

// healPivotCluster logs when reconnected
func (r *Reporter) healPivotCluster() {
	if !r.pivotHealthy {
		log.Info("connected with pivot cluster")
	}
	r.pivotHealthy = true
}
