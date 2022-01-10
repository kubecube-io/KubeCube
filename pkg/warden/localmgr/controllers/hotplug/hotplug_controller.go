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
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hotplugv1 "github.com/kubecube-io/kubecube/pkg/apis/hotplug/v1"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/warden/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubecube-io/kubecube/pkg/clog"
)

const (
	common   = "common"
	fail     = "fail"
	success  = "success"
	enabled  = "enabled"
	disabled = "disabled"
)

var _ reconcile.Reconciler = &HotplugReconciler{}

// HotplugReconciler reconciles a Hotplug object
type HotplugReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func newReconciler(mgr manager.Manager) (*HotplugReconciler, error) {
	r := &HotplugReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
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
			if errors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
			log.Error("get common hotplug fail, %v", err)
			return ctrl.Result{}, err
		}
		err = h.Client.Get(ctx, types.NamespacedName{Name: utils.Cluster}, &clusterConfig)
		if err != nil {
			if !errors.IsNotFound(err) {
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
			if errors.IsNotFound(err) {
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
		// audit not use helm
		if name == "audit" {
			addSuccessResult(result, fmt.Sprintf("audit is %v", c.Status))
			continue
		}

		// release status
		isReleaseExist := false
		release, err := helm.Status(namespace, name)
		if err != nil {
			log.Info("%v can not get status from release info, %v", name, err)
			isReleaseExist = false
		} else {
			if release.Info.Status == "deployed" {
				isReleaseExist = true
			}
		}

		// start helm doing
		if !isReleaseExist {
			if c.Status != enabled {
				// release no exist & disabled, do nothing
				addSuccessResult(result, "uninstalled")
				continue
			} else {
				// release no exist & enable, need install
				envs, err := YamlStringToJson(c.Env)
				if err != nil {
					addFailResult(result, fmt.Sprintf("parse env yaml fail, %v", err))
					continue
				}
				_, err = helm.Install(namespace, name, c.PkgName, envs)
				if err != nil {
					addFailResult(result, fmt.Sprintf("helm install fail, %v", err))
					continue
				}
				addSuccessResult(result, "helm install success")
			}
		} else {
			if c.Status != enabled {
				// release exist & disabled, need uninstall
				err := helm.Uninstall(namespace, name)
				if err != nil {
					addFailResult(result, fmt.Sprintf("helm uninstall fail, %v", err))
					continue
				}
				addSuccessResult(result, "helm uninstall success")
			} else {
				// release exist & enabled, need upgrade
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
					addSuccessResult(result, "release is running")
					continue
				}
				_, err = helm.Upgrade(namespace, name, c.PkgName, envs)
				if err != nil {
					addFailResult(result, fmt.Sprintf("helm upgrade fail, %v", err))
					continue
				}
				addSuccessResult(result, "upgrade success")
			}
		}
	}

	phase := success
	for _, r := range results {
		if r.Result == fail {
			phase = fail
			continue
		}
		// todo: there must be another way to judgement if pivot cluster
		if utils.Cluster == constants.PivotCluster {
			updateConfigMap(ctx, h.Client, r)
		}
	}

	// update status
	commonConfig.Status.Phase = phase
	commonConfig.Status.Results = results
	err := h.Client.Status().Update(ctx, &commonConfig)
	if err != nil {
		log.Error("update common hotplug fail, %v", err)
		return ctrl.Result{}, err
	}
	if req.Name == utils.Cluster {
		clusterConfig.Status.Phase = phase
		clusterConfig.Status.Results = results
		err := h.Client.Status().Update(ctx, &clusterConfig)
		if err != nil {
			log.Error("update cluster %v hotplug fail, %v", req.Name, err)
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
	clog.Error("component:%s, message:%s", result.Name, message)
	result.Result = fail
	result.Message = message
}

// SetupWithManager sets up the controller with the Manager.
func SetupWithManager(mgr ctrl.Manager) error {
	r, err := newReconciler(mgr)
	if err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&hotplugv1.Hotplug{}).
		Complete(r)
}
