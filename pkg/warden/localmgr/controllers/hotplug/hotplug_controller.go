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
	"encoding/json"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hotplugv1 "github.com/kubecube-io/kubecube/pkg/apis/hotplug/v1"
	"github.com/kubecube-io/kubecube/pkg/warden/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubecube-io/kubecube/pkg/clog"
)

const (
	COMMON  = "common"
	FAIL    = "fail"
	ENABLED = "enabled"
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
func (r *HotplugReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := clog.WithName("controller").WithValues("hotplug", req.NamespacedName)
	// get hotplug info
	commonConfig := hotplugv1.Hotplug{}
	clusterConfig := hotplugv1.Hotplug{}
	hotplugConfig := hotplugv1.Hotplug{}
	switch req.Name {
	case COMMON:
		err := r.Client.Get(ctx, req.NamespacedName, &commonConfig)
		if err != nil {
			if errors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
			log.Error("get common hotplug fail, %v", err)
			return ctrl.Result{}, err
		}
		err = r.Client.Get(ctx, types.NamespacedName{Name: utils.Cluster}, &clusterConfig)
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
		err := r.Client.Get(ctx, req.NamespacedName, &clusterConfig)
		if err != nil {
			if errors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
			log.Error("get cluster hotplug fail, %v", err)
			return ctrl.Result{}, err
		}
		err = r.Client.Get(ctx, types.NamespacedName{Name: COMMON}, &commonConfig)
		if err != nil {
			log.Warn("get common hotplug fail, %v", err)
			return ctrl.Result{}, err
		}
		hotplugConfig = MergeHotplug(commonConfig, clusterConfig)
	default:
		log.Warn("this hotplug not match this cluster, %s != %s", req.Name, utils.Cluster)
		return ctrl.Result{}, nil
	}

	result := make(map[string]string)
	resultStatus := "running"
	helm := NewHelm()
	// is change
	for _, component := range hotplugConfig.Spec.Component {
		namespace := component.Namespace
		name := component.Name
		if name == "audit" {
			continue
		}
		isReleaseExist := false
		// release status
		release, err := helm.Status(namespace, name)
		if err != nil {
			log.Info("can not get status from release info, %v", err)
			isReleaseExist = false
		} else {
			if release.Info.Status == "deployed" {
				isReleaseExist = true
			}
		}

		if !isReleaseExist {
			if component.Status != ENABLED {
				// release no exist & disabled, do nothing
				result[name] = "disabled and uninstalled"
				continue
			} else {
				// release no exist & enable, need install
				envs, err := YamlStringToJson(component.Env)
				if err != nil {
					result[name] = fmt.Sprintf("error - parse env yaml fail, %v", err)
					resultStatus = FAIL
				}
				_, err = helm.Install(namespace, name, component.PkgName, envs)
				if err != nil {
					result[name] = fmt.Sprintf("error - helm install fail, %v", err)
					resultStatus = FAIL
					continue
				}
				result[name] = "helm install success"
			}
		} else {
			if component.Status != ENABLED {
				// release exist & disabled, need uninstall
				err := helm.Uninstall(namespace, name)
				if err != nil {
					result[name] = fmt.Sprintf("error - helm uninstall fail, %v", err)
					resultStatus = FAIL
					continue
				}
				result[name] = "helm uninstall success"
			} else {
				// release exist & enabled, need upgrade
				envs, err := YamlStringToJson(component.Env)
				if err != nil {
					result[name] = fmt.Sprintf("error - parse env yaml fail, %v", err)
					resultStatus = FAIL
					continue
				}
				// get release values
				values, err := helm.GetValues(namespace, name)
				if err != nil {
					result[name] = fmt.Sprintf("error - helm value from release fail, %v", err)
					resultStatus = FAIL
					continue
				}
				if JudgeJsonEqual(envs, values) {
					result[name] = "release is running"
					continue
				}
				_, err = helm.Upgrade(namespace, name, component.PkgName, envs)
				if err != nil {
					result[name] = fmt.Sprintf("upgrade fail, %v", err)
					continue
				}
				result[name] = "upgrade success"
			}
		}
	}

	json, err := json.Marshal(result)
	if err != nil {
		clog.Warn("can not convert result %v to json, %v", result, err)
		return ctrl.Result{}, nil
	}
	switch req.Name {
	case COMMON:
		commonConfig.Status.Phase = resultStatus
		commonConfig.Status.Message = string(json)
		err := r.Client.Status().Update(ctx, &commonConfig)
		if err != nil {
			log.Error("update common hotplug fail, %v", err)
		}
	case utils.Cluster:
		clusterConfig.Status.Phase = resultStatus
		clusterConfig.Status.Message = string(json)
		err := r.Client.Status().Update(ctx, &clusterConfig)
		if err != nil {
			log.Error("update cluster hotplug fail, %v", err)
		}
	default:
		log.Error("this hotplug not match this cluster, %s != %s", req.Name, utils.Cluster)
	}
	log.Info(string(json))
	return ctrl.Result{}, nil
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
