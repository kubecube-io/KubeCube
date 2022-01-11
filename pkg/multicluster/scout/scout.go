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

package scout

import (
	"context"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils"
)

const (
	defaultInitialDelaySeconds = 10
	defaultWaitTimeoutSeconds  = 10
)

// Scout collects information from warden
type Scout struct {
	// LastHeartbeat record last heartbeat form warden reporter
	LastHeartbeat time.Time

	// WaitTimeoutSeconds that heartbeat not receive timeout
	WaitTimeoutSeconds int

	// InitialDelaySeconds the time that wait for warden start
	InitialDelaySeconds int

	// Cluster the cluster where the warden watch for
	Cluster string

	// Receiver receive warden info form api
	Receiver chan WardenInfo

	// StopCh use to stop scout for
	StopCh chan struct{}

	// Once ensure scout for be called once
	Once *sync.Once

	// client k8s client
	client client.Client

	// clusterState shows the real-time status for cluster
	clusterState v1.ClusterState
}

// WardenInfo contains intelligence within communication
type WardenInfo struct {
	// Cluster is cluster name the warden reported
	Cluster string `json:"cluster"`

	// ReportTime the time warden start to report
	ReportTime time.Time `json:"reportTime"`
}

func NewScout(cluster string, initialDelay, waitTimeoutSeconds int, cli client.Client, stopCh chan struct{}) *Scout {
	if initialDelay == 0 {
		initialDelay = defaultInitialDelaySeconds
	}
	if waitTimeoutSeconds == 0 {
		waitTimeoutSeconds = defaultWaitTimeoutSeconds
	}

	s := &Scout{
		Cluster:             cluster,
		Receiver:            make(chan WardenInfo),
		InitialDelaySeconds: initialDelay,
		WaitTimeoutSeconds:  waitTimeoutSeconds,
		client:              cli,
		StopCh:              stopCh,
		Once:                &sync.Once{},
	}

	// cluster processing means all things ready wait for warden startup
	s.clusterState = v1.ClusterProcessing

	return s
}

func (s *Scout) ClusterHealth() v1.ClusterState {
	return s.clusterState
}

// Collect will scout a specified warden of cluster
func (s *Scout) Collect(ctx context.Context) {
	for {
		select {
		case info := <-s.Receiver:
			s.healthWarden(ctx, info)

		case <-time.Tick(time.Duration(s.WaitTimeoutSeconds) * time.Second):
			s.illWarden(ctx)

		case <-ctx.Done():
			clog.Warn("scout of %v warden stopped: %v", s.Cluster, ctx.Err())
			return
		}
	}
}

// healthWarden do callback when receive heartbeat first
// todo(weilaaa): populate network delay with watden info
func (s *Scout) healthWarden(ctx context.Context, info WardenInfo) {
	cluster := &v1.Cluster{}
	err := s.client.Get(ctx, types.NamespacedName{Name: s.Cluster}, cluster)
	if err != nil {
		clog.Error(err.Error())
		return
	}

	if s.clusterState != v1.ClusterNormal {
		clog.Info("cluster %v connected", cluster.Name)
	}

	s.LastHeartbeat = time.Now()

	updateFn := func(obj *v1.Cluster) {
		state := v1.ClusterNormal
		obj.Status.State = &state
		obj.Status.Reason = fmt.Sprintf("receive heartbeat from cluster %s", s.Cluster)
		obj.Status.LastHeartbeat = &metav1.Time{Time: s.LastHeartbeat}
	}

	err = utils.UpdateClusterStatus(ctx, s.client, cluster, updateFn)
	if err != nil {
		clog.Error(err.Error())
		return
	}

	s.clusterState = v1.ClusterNormal
}

// illWarden do callback when warden ill
func (s *Scout) illWarden(ctx context.Context) {
	cluster := &v1.Cluster{}
	err := s.client.Get(ctx, types.NamespacedName{Name: s.Cluster}, cluster)
	if err != nil {
		clog.Error(err.Error())
	}

	if !isDisconnected(cluster, s.WaitTimeoutSeconds) {
		// going here means cluster heartbeat is normal

		if s.clusterState != v1.ClusterNormal {
			clog.Info("cluster %v connected", cluster.Name)
		}

		s.LastHeartbeat = cluster.Status.LastHeartbeat.Time
		s.clusterState = v1.ClusterNormal
		return
	}

	if s.clusterState == v1.ClusterNormal {
		reason := fmt.Sprintf("cluster %s disconnected", s.Cluster)

		updateFn := func(obj *v1.Cluster) {
			state := v1.ClusterAbnormal
			obj.Status.State = &state
			obj.Status.Reason = reason
			obj.Status.LastHeartbeat = &metav1.Time{Time: s.LastHeartbeat}
		}

		clog.Warn("%v, last heartbeat: %v", reason, s.LastHeartbeat)

		err := utils.UpdateClusterStatus(ctx, s.client, cluster, updateFn)
		if err != nil {
			clog.Error(err.Error())
		}
	}

	s.clusterState = v1.ClusterAbnormal
}

// isDisconnected determines the health of the cluster
// todo: consider use release do that
func isDisconnected(cluster *v1.Cluster, waitTimeoutSecond int) bool {
	clog.Debug("cluster %v, status: %v, time now: %v", cluster.Name, *cluster.Status.State, time.Now().String())

	// has no LastHeartbeat return directly
	if cluster.Status.LastHeartbeat == nil {
		return true
	}

	// if sub time less than timeout setting, we consider the cluster is healthy
	v := time.Now().Sub(cluster.Status.LastHeartbeat.Time)
	if v.Milliseconds() < (time.Duration(waitTimeoutSecond) * time.Second).Milliseconds() {
		return false
	}

	return true
}
