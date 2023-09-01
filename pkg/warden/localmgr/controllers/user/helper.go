package controllers

import (
	"context"
	"fmt"

	v1 "github.com/kubecube-io/kubecube/pkg/apis/user/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func addUserToTenant(user *v1.User, tenant string) {
	tenantSet := sets.NewString(user.Status.BelongTenants...)
	tenantSet.Insert(tenant)
	user.Status.BelongTenants = tenantSet.List()
	clog.Info("ensure user %v belongs to tenant %v", user.Name, tenant)
}

func addUserToProject(user *v1.User, project string) {
	projectSet := sets.NewString(user.Status.BelongProjects...)
	projectSet.Insert(project)
	user.Status.BelongProjects = projectSet.List()
	clog.Info("ensure user %v belongs to project %v", user.Name, project)
}

func appointUserAdmin(user *v1.User) {
	user.Status.PlatformAdmin = true
	clog.Info("appoint user %v is platform admin", user.Name)
}

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
