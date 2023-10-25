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

package syncmgr

import (
	"context"
	"strings"

	"github.com/kubecube-io/kubecube/pkg/clog"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Gc struct {
	client.Client
	cfg *rest.Config
}

func NewGc(cfg *rest.Config, client client.Client) *Gc {
	return &Gc{
		cfg:    cfg,
		Client: client,
	}
}

// GcWork gc work
// If a resource exists on the current cluster but not on the control cluster, the resource is a residual resource and needs to be deleted
func (g *Gc) GcWork() {
	pivotClient, err := client.New(g.cfg, client.Options{Scheme: scheme})
	if err != nil {
		clog.Fatal("error new pivot client: %s", err.Error())
	}
	for _, r := range syncListResources {
		err := g.List(context.Background(), r)
		if err != nil {
			clog.Warn("error list resource: %s", err.Error())
			continue
		}
		list := &unstructured.UnstructuredList{}
		err = scheme.Convert(r, list, nil)
		if err != nil {
			clog.Warn("error convert resource: %s", err.Error())
			continue
		}
		for _, item := range list.Items {
			if !isSyncResource(&item) {
				continue
			}
			u := unstructured.Unstructured{}
			u.SetNamespace(item.GetNamespace())
			u.SetName(item.GetName())
			kinds, _, err := scheme.ObjectKinds(r)
			if err != nil {
				clog.Warn("error get object kinds: %s", err.Error())
				continue
			}
			u.SetAPIVersion(kinds[0].GroupVersion().String())
			kind := strings.TrimSuffix(kinds[0].Kind, "List")
			u.SetKind(kind)

			err = pivotClient.Get(context.Background(), client.ObjectKeyFromObject(&u), &u)
			if err != nil {
				if errors.IsNotFound(err) {
					clog.Info("the resource %s/%s/%s/%s is not found on the pivot cluster, delete it", u.GetAPIVersion(), u.GetKind(), u.GetNamespace(), u.GetName())
					err = g.Delete(context.Background(), &u)
					if err != nil {
						clog.Warn("error delete resource: %s", err.Error())
					}
				} else {
					clog.Warn("error get resource: %s", err.Error())
				}
			}
		}
	}
}
