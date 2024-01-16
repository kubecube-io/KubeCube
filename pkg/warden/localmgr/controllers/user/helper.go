/*
Copyright 2023 KubeCube Authors

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

package controllers

import (
	"context"
	"fmt"
	"strings"

	v1 "github.com/kubecube-io/kubecube/pkg/apis/user/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
		newUser.Status.BelongProjectInfos = user.Status.BelongProjectInfos

		err = cli.Status().Update(ctx, newUser)
		if err != nil {
			return err
		}
		return nil
	})
}

func createObjOrUpdateObjLabels(ctx context.Context, cli client.Client, obj client.Object) error {
	labels := obj.GetLabels()
	_, err := controllerutil.CreateOrUpdate(ctx, cli, obj, func() error {
		obj.SetLabels(labels)
		return nil
	})
	return err
}

func updateUserStatusErrStr(user string, err error) string {
	return fmt.Sprintf("update user %v status failed: %v", user, err)
}

func isGenBinding(name string) bool {
	return strings.HasPrefix(name, "gen-")
}
