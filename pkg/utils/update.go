package utils

import (
	"context"

	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
)

func UpdateStatus(ctx context.Context, cli client.Client, currentCluster *clusterv1.Cluster, updateFn func(cluster *clusterv1.Cluster)) error {
	memberClusterCopy := currentCluster.DeepCopy()

	updateFn(memberClusterCopy)

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := cli.Status().Update(ctx, memberClusterCopy, &client.UpdateOptions{})
		if err != nil {
			return err
		}
		return nil
	})
}
