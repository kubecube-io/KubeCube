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

package utils

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
)

func UpdateClusterStatus(ctx context.Context, cli client.Client, cluster *clusterv1.Cluster, updateFn func(cluster *clusterv1.Cluster)) error {
	updateFn(cluster)

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		newCluster := &clusterv1.Cluster{}
		err := cli.Get(ctx, types.NamespacedName{Name: cluster.Name}, newCluster)
		if err != nil {
			return err
		}

		newCluster.Status = cluster.Status

		err = cli.Status().Update(ctx, newCluster, &client.UpdateOptions{})
		if err != nil {
			return err
		}
		return nil
	})
}

func UpdateClusterStatusByState(ctx context.Context, cli client.Client, cluster *clusterv1.Cluster, state clusterv1.ClusterState) error {
	updateFn := func(cluster *clusterv1.Cluster) {
		reason := fmt.Sprintf("cluster(%v) is %s", cluster.Name, state)
		cluster.Status.State = &state
		cluster.Status.Reason = reason
	}

	return UpdateClusterStatus(ctx, cli, cluster, updateFn)
}
