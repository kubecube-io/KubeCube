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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	tenantv1 "github.com/kubecube-io/kubecube/pkg/apis/tenant/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/env"
)

var _ reconcile.Reconciler = &ProjectReconciler{}

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

	needUpdate := false

	// if .spec.namespace is nil, add .spec.namespace
	nsName := constants.ProjectNsPrefix + req.Name
	if project.Spec.Namespace != nsName {
		project.Spec.Namespace = nsName
		needUpdate = true
	}

	// if annotation not content kubecube.io/sync, add it
	ano := project.Annotations
	if ano == nil {
		ano = make(map[string]string)
	}
	if _, ok := ano[constants.SyncAnnotation]; !ok {
		ano[constants.SyncAnnotation] = "1"
		project.Annotations = ano
		needUpdate = true
	}

	if needUpdate {
		err = r.Client.Update(ctx, &project)
		if err != nil {
			log.Error("update project fail, %v", err)
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

	if env.CreateHNCNs() {
		err = r.crateProjectNamespace(ctx, tenantName, project.Name)
		if err != nil {
			clog.Error(err.Error())
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *ProjectReconciler) deleteProject(projectName string) (ctrl.Result, error) {
	// delete subNamespace of project
	err := r.deleteSubNSOfProject(projectName)
	if err != nil {
		clog.Error(err.Error())
		return ctrl.Result{}, err
	}

	projectNs := &v1.Namespace{}
	projectNs.Name = constants.ProjectNsPrefix + projectName

	err = r.Delete(context.Background(), projectNs)
	if err != nil && errors.IsNotFound(err) {
		clog.Error(err.Error())
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ProjectReconciler) deleteSubNSOfProject(projectName string) error {
	// this label will list sub ns under project both spawned ns of project
	lbSelector, err := labels.Parse(fmt.Sprintf("%v=%v", constants.HncProjectLabel, projectName))
	if err != nil {
		return err
	}

	nsList := &v1.NamespaceList{}
	err = r.List(context.TODO(), nsList, &client.ListOptions{LabelSelector: lbSelector})
	if err != nil {
		return err
	}

	errs := []error{}

	for _, ns := range nsList.Items {
		err = r.Delete(context.TODO(), ns.DeepCopy())
		if err != nil && !errors.IsNotFound(err) {
			err = fmt.Errorf("delete ns %v under project %v failed: %v", ns.Name, projectName, err)
			errs = append(errs, err)
			continue
		}
	}

	if len(errs) > 0 {
		return utilerrors.NewAggregate(errs)
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

func (r *ProjectReconciler) crateProjectNamespace(ctx context.Context, tenant, project string) error {
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("kubecube-project-%v", project),
			Annotations: map[string]string{constants.HncAnnotation: fmt.Sprintf("kubecube-tenant-%v", tenant)},
			Labels: map[string]string{
				constants.HncIncludedNsLabel:                                        "true",
				fmt.Sprintf("kubecube-project-%v.tree.hnc.x-k8s.io/depth", project): "0",
				fmt.Sprintf("kubecube-tenant-%v.tree.hnc.x-k8s.io/depth", tenant):   "1",
				constants.HncProjectLabel:                                           project,
				constants.HncTenantLabel:                                            tenant,
			},
		},
	}

	err := r.Create(ctx, ns)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}
