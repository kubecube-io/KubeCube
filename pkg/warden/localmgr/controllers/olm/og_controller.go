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

package olm

import (
	"context"
	"fmt"
	"strings"

	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"

	v1 "github.com/operator-framework/api/pkg/operators/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	log clog.CubeLogger

	_ reconcile.Reconciler = &OperatorGroupReconciler{}
)

type OperatorGroupReconciler struct {
	client.Client
	RESTMapper meta.RESTMapper
}

func newReconciler(mgr manager.Manager) (*OperatorGroupReconciler, error) {
	log = clog.WithName("operatorGroup")

	r := &OperatorGroupReconciler{
		Client:     mgr.GetClient(),
		RESTMapper: mgr.GetRESTMapper(),
	}

	return r, nil
}

// Reconcile that reconcile OperatorGroup event but write nothing
// to OperatorGroup cr, all things we do with OG are read-only
func (r *OperatorGroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log.Info("Reconcile OperatorGroup (%v/%v)", req.Namespace, req.Name)

	og := &v1.OperatorGroup{}

	err := r.Client.Get(ctx, req.NamespacedName, og)
	if err != nil {
		if errors.IsNotFound(err) {
			// do nothing when OperatorGroup has been deleted
			// meanwhile related ClusterRole would be to deleted
			// by olm controller
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	return r.sync(og)
}

func (r *OperatorGroupReconciler) sync(og *v1.OperatorGroup) (ctrl.Result, error) {
	// get ClusterRole admin for OperatorGroup
	ogClusterRole := &rbacv1.ClusterRole{}
	err := r.Get(context.Background(), client.ObjectKey{Name: og.Name + "-admin"}, ogClusterRole)
	if err != nil {
		// whatever error is not found or what, we need requeue the event
		return ctrl.Result{}, err
	}

	log.Debug("found related ClusterRole(%v) with OperatorGroup(%v/%v)", ogClusterRole.Name, og.Namespace, og.Name)

	if ogClusterRole.AggregationRule == nil {
		log.Warn("OperatorGroup(%v/%v) has no aggregation rules", og.Namespace, og.Name)
		return ctrl.Result{}, nil
	}

	// aggregate og ClusterRole to platform-admin, tenant-admin, project-admin
	labels := ogClusterRole.GetLabels()
	labels[constants.PlatformAdminAgLabel] = "true"
	labels[constants.TenantAdminAgLabel] = "true"
	labels[constants.ProjectAdminAgLabel] = "true"

	ogClusterRole.Labels = labels

	// update og ClusterRole to make sense
	return ctrl.Result{}, retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := r.Update(context.Background(), ogClusterRole)
		if err != nil {
			return err
		}
		return nil
	})
}

// findRelatedUser would find out related users with given namespaces
func (r *OperatorGroupReconciler) findRelatedUsers(nss []string) ([]string, error) {
	if len(nss) == 0 {
		// skip reconcile when nss is empty
		return nil, nil
	}

	var err error

	for _, ns := range nss {
		nsInst := &corev1.Namespace{}
		err = r.Get(context.Background(), client.ObjectKey{Name: ns}, nsInst)
		if err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func (r *OperatorGroupReconciler) createOrUpdateClusterRole(obj *rbacv1.ClusterRole) error {
	objCopy := obj.DeepCopy()
	res, err := controllerutil.CreateOrUpdate(context.Background(), r.Client, objCopy, func() error {
		objCopy.Labels = obj.Labels
		objCopy.Annotations = obj.Annotations
		objCopy.Rules = obj.Rules
		objCopy.AggregationRule = obj.AggregationRule
		return nil
	})
	if err != nil {
		return err
	}

	if res == controllerutil.OperationResultCreated {
		log.Info("Create ClusterRole %v successed", obj.GetName())
	} else if res == controllerutil.OperationResultUpdated {
		log.Info("Update ClusterRole %v successed", obj.GetName())
	} else {
		log.Info("ClusterRole %v is up to date", obj.GetName())
	}

	return nil
}

// makeClusterRole generate a ClusterRole for the given OperatorGroup
func (r *OperatorGroupReconciler) makeClusterRole(og *v1.OperatorGroup) (*rbacv1.ClusterRole, error) {
	providedAPIS, ok := og.Annotations["olm.providedAPIs"]
	if !ok {
		log.Warn("OperatorGroup(%v/%v) has no annotation [olm.providedAPIs]", og.Namespace, og.Name)
		// skip OG without annotation olm.providedAPIs
		return nil, nil
	}

	rules, err := r.parseProvideAPis(providedAPIS)
	if err != nil {
		return nil, err
	}

	labels := map[string]string{"olm.operatorgourp": fmt.Sprintf("%v/%v", og.Namespace, og.Name)}

	// ClusterRole name sample: og.og-single.default
	name := "og." + og.Name + "." + og.Namespace

	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Rules:      rules,
	}, nil
}

// parseProvideAPis transfer provideAPis to rbac policy rules
func (r *OperatorGroupReconciler) parseProvideAPis(s string) ([]rbacv1.PolicyRule, error) {
	if len(s) == 0 {
		return nil, fmt.Errorf("provideAPIs should not be empty")
	}

	// transfer kind to resource
	// gvk sample: EtcdBackup.v1beta2.etcd.database.coreos.com
	transFn := func(gvk string) (string, error) {
		strs := strings.SplitN(gvk, ".", 3)
		if len(strs) != 3 {
			return "", fmt.Errorf("ProvideAPis format error: %v", gvk)
		}

		gk := schema.GroupKind{
			Group: strs[2],
			Kind:  strs[0],
		}

		m, err := r.RESTMapper.RESTMapping(gk, strs[1])
		if err != nil {
			return "", err
		}

		return m.Resource.Resource, nil
	}

	splitAPIS := strings.Split(s, ",")
	resources := sets.NewString()

	for _, splitAPI := range splitAPIS {
		// transfer error will interrupt process
		resource, err := transFn(splitAPI)
		if err != nil {
			return nil, err
		}
		resources.Insert(resource)
	}

	log.Info("transferred provided apis: %v", resources.List())

	// we use one policy rule to container all of provideAPis
	// apiGroups: *
	// resources: provideAPis
	// verbs: *
	return []rbacv1.PolicyRule{{
		APIGroups: []string{"*"},
		Resources: resources.List(),
		Verbs:     []string{"*"},
	}}, nil
}

func SetupWithManager(mgr ctrl.Manager) error {
	r, err := newReconciler(mgr)
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).For(&v1.OperatorGroup{}).Complete(r)
}
