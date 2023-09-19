package controllers

import (
	"context"
	"fmt"
	"strings"

	v1 "github.com/kubecube-io/kubecube/pkg/apis/user/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func updateUserStatus(ctx context.Context, cli client.Client, user *v1.User) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		newUser := &v1.User{}
		err := cli.Get(ctx, types.NamespacedName{Name: user.Name}, newUser)
		if err != nil {
			return err
		}

		// update status here
		newUser.Status.PlatformAdmin = user.Status.PlatformAdmin
		newUser.Status.BelongTenants = user.Status.BelongTenants
		newUser.Status.BelongProjects = user.Status.BelongProjects

		err = cli.Status().Update(ctx, newUser)
		if err != nil {
			return err
		}
		return nil
	})
}

func updateUserStatusErrStr(user string, err error) string {
	return fmt.Sprintf("update user %v status failed: %v", user, err)
}

func isGenBinding(name string) bool {
	return strings.HasPrefix(name, "gen-")
}
