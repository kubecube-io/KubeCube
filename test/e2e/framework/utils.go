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

package framework

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"

	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
)

// Create namespace
func CreateNamespace(baseName string) (*v1.Namespace, error) {
	cli := clients.Interface().Kubernetes(constants.LocalCluster)
	labels := map[string]string{
		"e2e-run":       string(uuid.NewUUID()),
		"e2e-framework": baseName,
	}
	name := fmt.Sprintf("kubecube-e2etest-%v-%v", baseName, strconv.Itoa(rand.Intn(10000)))
	namespaceObj := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
	if err := wait.PollImmediate(WaitInterval, WaitTimeout, func() (bool, error) {
		err := cli.Direct().Create(context.TODO(), namespaceObj)
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				// regenerate on conflict
				clog.Info("Namespace name %q was already taken, generate a new name and retry", namespaceObj.Name)
				namespaceObj.Name = fmt.Sprintf("kubecube-e2etest-%v-%v", name, strconv.Itoa(rand.Intn(10000)))
			} else {
				clog.Info("Unexpected error while creating namespace: %v", err)
			}
			return false, nil
		}
		return true, nil
	}); err != nil {
		return nil, err
	}

	return namespaceObj, nil
}

// Delete Namespace
func DeleteNamespace(ns *v1.Namespace) error {
	cli := clients.Interface().Kubernetes(constants.LocalCluster)
	err := cli.Direct().Delete(context.TODO(), ns)
	if err != nil && !apierrors.IsNotFound(err) {
		clog.Error("error deleting namespace %s: %v", ns.Name, err)
		return err
	}
	if err = wait.Poll(WaitInterval, WaitTimeout,
		func() (bool, error) {
			var nsTemp v1.Namespace
			err := cli.Direct().Get(context.TODO(), types.NamespacedName{Name: ns.Name}, &nsTemp)
			if err != nil {
				if apierrors.IsNotFound(err) {
					return true, nil
				}
				return false, nil
			}
			return false, nil
		}); err != nil {
		return err
	}
	return nil
}
