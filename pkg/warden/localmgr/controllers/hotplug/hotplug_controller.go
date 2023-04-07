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

package hotplug

import (
	"context"
	"errors"
	"fmt"

	helmrelease "helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	apierros "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hotplugv1 "github.com/kubecube-io/kubecube/pkg/apis/hotplug/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/warden/utils"
)

const (
	common  = "common"
	fail    = "fail"
	success = "success"
	enabled = "enabled"
	//disabled = "disabled"
)

var _ reconcile.Reconciler = &HotplugReconciler{}

// HotplugReconciler reconciles a Hotplug object
type HotplugReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	isMemberCluster bool
	clusterName     string
}

func newReconciler(mgr manager.Manager, isMemberCluster bool, clusterName string) (*HotplugReconciler, error) {
	r := &HotplugReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		isMemberCluster: isMemberCluster,
		clusterName:     clusterName,
	}
	return r, nil
}

//+kubebuilder:rbac:groups=hotplug.kubecube.io,resources=hotplugs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hotplug.kubecube.io,resources=hotplugs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hotplug.kubecube.io,resources=hotplugs/finalizers,verbs=update

// list/watch hotplug config and helm install/upgrade components
// 1、get change config and parse
// 2、judge is change
// 3、install/upgrade component
func (h *HotplugReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := clog.WithName("controller").WithValues("hotplug", req.NamespacedName)
	// get hotplug info
	commonConfig := hotplugv1.Hotplug{}
	clusterConfig := hotplugv1.Hotplug{}
	hotplugConfig := hotplugv1.Hotplug{}
	switch req.Name {
	case common:
		err := h.Client.Get(ctx, req.NamespacedName, &commonConfig)
		if err != nil {
			if apierros.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
			log.Error("get common hotplug fail, %v", err)
			return ctrl.Result{}, err
		}
		err = h.Client.Get(ctx, types.NamespacedName{Name: utils.Cluster}, &clusterConfig)
		if err != nil {
			if !apierros.IsNotFound(err) {
				log.Error("get cluster hotplug fail, %v", err)
				return ctrl.Result{}, err
			}
			// cluster config is nil
			hotplugConfig = MergeHotplug(commonConfig, clusterConfig)
		} else {
			hotplugConfig = MergeHotplug(commonConfig, clusterConfig)
		}
	case utils.Cluster:
		err := h.Client.Get(ctx, req.NamespacedName, &clusterConfig)
		if err != nil {
			if apierros.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
			log.Error("get cluster hotplug fail, %v", err)
			return ctrl.Result{}, err
		}
		err = h.Client.Get(ctx, types.NamespacedName{Name: common}, &commonConfig)
		if err != nil {
			log.Warn("get common hotplug fail, %v", err)
			return ctrl.Result{}, err
		}
		hotplugConfig = MergeHotplug(commonConfig, clusterConfig)
	default:
		log.Warn("this hotplug not match this cluster, %s != %s", req.Name, utils.Cluster)
		return ctrl.Result{}, nil
	}

	// helm do
	results := []*hotplugv1.DeployResult{}
	helm := NewHelm()
	for _, c := range hotplugConfig.Spec.Component {
		namespace := c.Namespace
		name := c.Name
		result := &hotplugv1.DeployResult{Name: name, Status: c.Status}
		results = append(results, result)

		// query release status
		isReleaseExist := false
		release, err := helm.Status(namespace, name)
		if err != nil {
			if !errors.As(err, &driver.ErrReleaseNotFound) {
				log.Info("get release %v failed: %v", name, err)
				return ctrl.Result{}, err
			}
		}

		// any way we found release about chart, we think it exists
		if release != nil && release.Info.Status != helmrelease.StatusUninstalled {
			isReleaseExist = true
			log.Info("release (%v/%v) exist and status is %v", release.Name, release.Namespace, release.Info.Status)
		}

		switch {
		case !isReleaseExist && c.Status != enabled: // release no exist & disabled, do nothing
			addSuccessResult(result, "clear")
			continue
		case !isReleaseExist && c.Status == enabled: // release no exist & enable, need install
			envs, err := YamlStringToJson(c.Env)
			if err != nil {
				addFailResult(result, fmt.Sprintf("parse env yaml fail, %v", err))
				continue
			}
			log.Info("install helm chart (%v/%v)", name, namespace)
			_, err = helm.Install(namespace, name, c.PkgName, envs)
			if err != nil {
				log.Info("install helm chart (%v/%v) failed: %v", name, namespace, err)
				addFailResult(result, fmt.Sprintf("helm install fail, %v", err))
				continue
			}
			addSuccessResult(result, "helm install success")
		case isReleaseExist && c.Status != enabled: // release exist & disabled, need uninstall
			log.Info("uninstall helm chart (%v/%v)", name, namespace)
			err := helm.Uninstall(namespace, name)
			if err != nil {
				log.Info("uninstall helm chart (%v/%v) failed", name, namespace, err)
				addFailResult(result, fmt.Sprintf("helm uninstall fail, %v", err))
				continue
			}
			addSuccessResult(result, "helm uninstall success")
		case isReleaseExist && c.Status == enabled: // release exist & enabled, need upgrade
			envs, err := YamlStringToJson(c.Env)
			if err != nil {
				addFailResult(result, fmt.Sprintf("parse env yaml fail, %v", err))
				continue
			}
			// get release values
			values, err := helm.GetValues(namespace, name)
			if err != nil {
				addFailResult(result, fmt.Sprintf("helm get values fail, %v", err))
				continue
			}
			if JudgeJsonEqual(envs, values) {
				if release.Info.Status != helmrelease.StatusDeployed {
					addSuccessResult(result, "release is existing but status not ok please check")
				} else {
					addSuccessResult(result, "release is running")
				}
				continue
			}
			log.Info("upgrade helm chart (%v/%v)", name, namespace)
			_, err = helm.Upgrade(namespace, name, c.PkgName, envs)
			if err != nil {
				log.Info("upgrade helm chart (%v/%v) failed", name, namespace, err)
				addFailResult(result, fmt.Sprintf("helm upgrade fail, %v", err))
				continue
			}
			addSuccessResult(result, "upgrade success")
		}
	}

	// set final status phase of hotplug
	phase := success
	for _, r := range results {
		if r.Result == fail {
			phase = fail
			continue
		}
	}

	// update feature configmap if need
	if !h.isMemberCluster {
		if err := updateConfigMap(ctx, h.Client, results); err != nil {
			log.Warn("update feature config failed and will keep retry: %v", err)
			return ctrl.Result{}, err
		}
	}

	// update status
	commonConfig.Status.Phase = phase
	commonConfig.Status.Results = results
	err := h.Client.Status().Update(ctx, &commonConfig)
	if err != nil {
		log.Warn("update common hotplug fail and will keep retry: %v", err)
		return ctrl.Result{}, err
	}
	if req.Name == utils.Cluster {
		clusterConfig.Status.Phase = phase
		clusterConfig.Status.Results = results
		err := h.Client.Status().Update(ctx, &clusterConfig)
		if err != nil {
			log.Warn("update cluster %v hotplug fail and will keep retry: %v", req.Name, err)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func addSuccessResult(result *hotplugv1.DeployResult, message string) {
	clog.Info("component:%s, message:%s", result.Name, message)
	result.Result = success
	result.Message = message
}

func addFailResult(result *hotplugv1.DeployResult, message string) {
	clog.Warn("component:%s, message:%s", result.Name, message)
	result.Result = fail
	result.Message = message
}

// SetupWithManager sets up the controller with the Manager.
func SetupWithManager(mgr ctrl.Manager, isMemberCluster bool, clusterName string) error {
	r, err := newReconciler(mgr, isMemberCluster, clusterName)
	if err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&hotplugv1.Hotplug{}).
		Complete(r)
}
