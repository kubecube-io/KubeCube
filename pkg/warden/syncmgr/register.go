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

package syncmgr

import (
	"context"
	"strconv"

	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	Skip   = "skip"
	Create = "create"
	Update = "update"
	Delete = "delete"

	// pivot resource version key for compare with local resource
	pivotResourceVersion = "pivotResourceVersion"
)

/*
1. native resource should not be affected?
2. what if sync resource changed manually in member cluster? use webhook?
3. update operation is valid?
4. how to record log?
5. use reflect of not to copy interface value?
*/
// setupCtrlWithManager add reconcile func for each sync resource
// resync reference to https://github.com/cloudnativeto/sig-kubernetes/issues/11
func (s *SyncManager) setupCtrlWithManager(resource client.Object) error {
	pivotClient := s.Manager.GetClient()
	localClient := s.LocalClient

	r := reconcile.Func(func(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
		var (
			action = Skip
			err    error
			obj    = resource
		)

		// record sync log
		defer func() {
			log.Info("sync: %s %v, name: %v, namespace: %v, err: %v", action, obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), obj.GetNamespace(), err)
		}()

		if err = pivotClient.Get(ctx, req.NamespacedName, obj); err != nil {
			// delete: when object is not exist in pivot cluster
			if errors.IsNotFound(err) {
				action = Delete
				err = localClient.Delete(ctx, obj, &client.DeleteOptions{})
				if err != nil {
					return reconcile.Result{}, err
				}
				return reconcile.Result{}, nil
			}
			return reconcile.Result{}, err
		}

		newObj, err := newGenericObj(obj)
		if err != nil {
			return reconcile.Result{}, err
		}

		trimObjMeta(obj)

		err = localClient.Get(ctx, req.NamespacedName, newObj)
		if err != nil {
			if errors.IsNotFound(err) {
				// create: when object is not exist in local cluster
				action = Create
				err = localClient.Create(ctx, obj, &client.CreateOptions{})
				if err != nil {
					return reconcile.Result{Requeue: true}, err
				}
				return reconcile.Result{}, nil
			}
			return reconcile.Result{}, err
		}

		pivotRsVersion, err := strconv.Atoi(obj.GetAnnotations()[pivotResourceVersion])
		if err != nil {
			return reconcile.Result{}, err
		}
		localRsVersion, err := strconv.Atoi(newObj.GetAnnotations()[pivotResourceVersion])
		if err != nil {
			return reconcile.Result{}, err
		}

		// update: when pivot resource version bigger than local resource version
		if pivotRsVersion > localRsVersion {
			action = Update
			obj.SetResourceVersion(newObj.GetResourceVersion())
			obj.SetUID(newObj.GetUID())
			err = localClient.Update(ctx, obj, &client.UpdateOptions{})
			if err != nil {
				return reconcile.Result{}, err
			}
		}

		return reconcile.Result{}, nil
	})

	return ctrl.NewControllerManagedBy(s).
		For(resource).
		WithEventFilter(eventPredicate()).
		Complete(r)
}

// trimObjMeta trim read-only field of obj metadata avoid of conflict
// and record resource version on pivot cluster.
func trimObjMeta(obj client.Object) {
	annotations := obj.GetAnnotations()
	annotations[pivotResourceVersion] = obj.GetResourceVersion()

	if _, ok := annotations["kubectl.kubernetes.io/last-applied-configuration"]; ok {
		delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
	}

	obj.SetAnnotations(annotations)
	obj.SetResourceVersion("")
}

// isSyncResource determined if sync that event
func isSyncResource(obj client.Object) bool {
	// resource inherited by hnc do not need sync
	if _, ok := obj.GetLabels()[constants.HncInherited]; ok {
		return false
	}

	if v, ok := obj.GetAnnotations()[constants.SyncAnnotation]; ok {
		b, err := strconv.ParseBool(v)
		if b && err == nil {
			return true
		}
		if err != nil {
			log.Error("value format of annotation %v failed: %v, got value: %v%", constants.SyncAnnotation, err, v)
		}
	}

	return false
}

// eventPredicate do event filter for reconcile
func eventPredicate() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			return isSyncResource(event.Object)
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			return isSyncResource(updateEvent.ObjectNew)
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return isSyncResource(deleteEvent.Object)
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	}
}
