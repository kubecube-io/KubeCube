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

package controllers

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/hierarchical-namespaces/api/v1alpha2"

	tenantv1 "github.com/kubecube-io/kubecube/pkg/apis/tenant/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/env"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	hnc "sigs.k8s.io/hierarchical-namespaces/api/v1alpha2"
)

var _ reconcile.Reconciler = &ProjectReconciler{}

const (
	// Default timeouts to be used in TimeoutContext
	waitInterval = 2 * time.Second
	waitTimeout  = 120 * time.Second
)

// ProjectReconciler reconciles a Project object
type ProjectReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func newReconciler(mgr manager.Manager) (*ProjectReconciler, error) {
	r := &ProjectReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	return r, nil
}

//+kubebuilder:rbac:groups=tenant.kubecube.io,resources=projects,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tenant.kubecube.io,resources=projects/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tenant.kubecube.io,resources=projects/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the Project object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.0/pkg/reconcile
func (r *ProjectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := clog.WithName("reconcile").WithValues("project", req.NamespacedName)

	// find project info, if the project-namespace is not exist, create one
	project := tenantv1.Project{}
	err := r.Client.Get(ctx, req.NamespacedName, &project)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.deleteProject(req.Name)
		}
		log.Warn("get project fail, %v", err)
		return ctrl.Result{}, nil
	}

	// if .spec.namespace is nil, add .spec.namespace
	nsName := constants.ProjectNsPrefix + req.Name
	if project.Spec.Namespace != nsName {
		project.Spec.Namespace = nsName
		err = r.Client.Update(ctx, &project)
		if err != nil {
			log.Error("update project .spec.namespace fail, %v", err)
			return ctrl.Result{}, err
		}
	}

	// if annotation not content kubecube.io/sync, add it
	ano := project.Annotations
	if ano == nil {
		ano = make(map[string]string)
	}
	if _, ok := ano[constants.SyncAnnotation]; !ok {
		ano[constants.SyncAnnotation] = "1"
		project.Annotations = ano
		err = r.Client.Update(ctx, &project)
		if err != nil {
			log.Error("update project .metadata.annotations fail, %v", err)
			return ctrl.Result{}, err
		}
	}

	// according to kubecube.io/tenant label get user
	labels := project.ObjectMeta.Labels
	tenantName, ok := labels[constants.TenantLabel]
	if !ok {
		log.Error("this project do not content tenant label .metadata.labels.kubecube.io/tenant")
		return ctrl.Result{}, fmt.Errorf("this project %s do not content tenant label", req.NamespacedName)
	}
	tenant := tenantv1.Tenant{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: tenantName}, &tenant)
	if err != nil {
		log.Error("get tenant by project labels fail, %v", err)
		return ctrl.Result{}, err
	}
	if tenant.Spec.Namespace == "" {
		log.Error("the tenant %s do not content .spec.namespace", tenantName)
		return ctrl.Result{}, fmt.Errorf("the tenant %s do not content .spec.namespace", tenantName)
	}

	subnamesapceAchor := hnc.SubnamespaceAnchor{}
	// Weather subnamespaceAchor exist
	err = r.Client.Get(ctx, types.NamespacedName{Namespace: tenant.Spec.Namespace, Name: project.Spec.Namespace}, &subnamesapceAchor)
	if err != nil {
		if !errors.IsNotFound(err) {
			log.Warn("create project subnamespaces fail, %v", err)
			return ctrl.Result{}, err
		}

		newSubnamesapceAchor := hnc.SubnamespaceAnchor{
			TypeMeta: metav1.TypeMeta{
				Kind:       "SubnamespaceAnchor",
				APIVersion: "hnc.x-k8s.io/v1alpha2",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      project.Spec.Namespace,
				Namespace: tenant.Spec.Namespace,
				Annotations: map[string]string{
					constants.SyncAnnotation: "1",
				},
			},
			Spec: hnc.SubnamespaceAnchorSpec{
				Labels: []hnc.MetaKVP{
					{
						Key:   constants.HncTenantLabel,
						Value: tenant.Name,
					},
					{
						Key:   constants.HncProjectLabel,
						Value: project.Name,
					},
				},
			},
		}
		err = r.Client.Create(ctx, &newSubnamesapceAchor)
		if err != nil {
			log.Warn("create project subnamespaces fail, %v", err)
			return ctrl.Result{}, err
		}
	}

	projectNs := corev1.Namespace{}
	projectNsName := constants.ProjectNsPrefix + project.Name
	err = r.Client.Get(ctx, types.NamespacedName{Name: projectNsName}, &projectNs)
	if err != nil && !errors.IsNotFound(err) {
		log.Error(err.Error())
		return ctrl.Result{}, err
	}
	if errors.IsNotFound(err) {
		err = wait.Poll(2*time.Second, 120*time.Second, func() (done bool, err error) {
			err = r.Client.Get(ctx, types.NamespacedName{Name: constants.ProjectNsPrefix + project.Name}, &projectNs)
			if err != nil {
				return false, nil
			} else {
				return true, nil
			}
		})
		if err != nil {
			log.Error("wait for hnc spread ns %v failed: %v", projectNsName, err)
			return ctrl.Result{}, err
		}
	}

	needUpdate := false
	if projectNs.Labels != nil {
		for k, v := range env.HncManagedLabels {
			if _, ok := projectNs.Labels[k]; ok && v == "-" {
				delete(projectNs.Labels, k)
				needUpdate = true
				continue
			}
			if projectNs.Labels[k] != v {
				projectNs.Labels[k] = v
				needUpdate = true
			}
		}
	} else {
		projectNs.Labels = env.EnsureManagedLabels(env.HncManagedLabels)
		needUpdate = true
	}

	if needUpdate {
		err = r.Client.Update(ctx, &projectNs, &client.UpdateOptions{})
		if err != nil {
			log.Warn("update project namespace %v labels failed: %v", projectNs.Name, err)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *ProjectReconciler) deleteProject(projectName string) (ctrl.Result, error) {
	// delete subNamespace of project
	err := r.deleteSubNSOfProject(projectName)
	return ctrl.Result{}, err
}

func (r *ProjectReconciler) deleteSubNSOfProject(projectName string) error {
	ctx := context.Background()
	name := constants.ProjectNsPrefix + projectName
	subNamespaceList := &v1alpha2.SubnamespaceAnchorList{}
	if err := r.Client.List(ctx, subNamespaceList); err != nil {
		if errors.IsNotFound(err) {
			return nil
		} else {
			clog.Error("get subnamespacelist err: %v", err)
			return fmt.Errorf("get subnamespace list err")
		}
	}
	for _, subNamespace := range subNamespaceList.Items {
		if subNamespace.Name != name {
			continue
		}
		namespace := subNamespace.Namespace
		if err := r.Client.Delete(ctx, &subNamespace); err != nil {
			clog.Error("delete subnamespace of project err: %v", err)
			return fmt.Errorf("delete subnamespace of project err")
		}
		err := wait.Poll(waitInterval, waitTimeout,
			func() (bool, error) {
				e := r.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &subNamespace)
				if errors.IsNotFound(e) {
					return true, nil
				} else {
					return false, nil
				}
			})
		if err != nil {
			clog.Error("wait for delete subnamespace of project err: %v", err)
			return fmt.Errorf("wait for delete subnamespace of project err")
		}
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func SetupWithManager(mgr ctrl.Manager) error {
	r, err := newReconciler(mgr)
	if err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&tenantv1.Project{}).
		Complete(r)
}
