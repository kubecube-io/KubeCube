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

package cache

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache/informertest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ cache.Cache = &FakeClient{}

// FakeClient implement cache.Cache
type FakeClient struct {
	Client client.Client
	Cache  *informertest.FakeInformers
}

func (c *FakeClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	return c.Client.Get(ctx, key, obj)
}

func (c *FakeClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return c.Client.List(ctx, list, opts...)
}

func (c *FakeClient) GetInformer(ctx context.Context, obj client.Object) (cache.Informer, error) {
	return c.Cache.GetInformer(ctx, obj)
}

func (c *FakeClient) GetInformerForKind(ctx context.Context, gvk schema.GroupVersionKind) (cache.Informer, error) {
	return c.Cache.GetInformerForKind(ctx, gvk)
}

func (c *FakeClient) Start(ctx context.Context) error {
	return c.Cache.Start(ctx)
}

func (c *FakeClient) WaitForCacheSync(ctx context.Context) bool {
	return c.Cache.WaitForCacheSync(ctx)
}

func (c *FakeClient) IndexField(ctx context.Context, obj client.Object, field string, extractValue client.IndexerFunc) error {
	return c.Cache.IndexField(ctx, obj, field, extractValue)
}
