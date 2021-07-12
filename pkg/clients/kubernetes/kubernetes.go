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

package kubernetes

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"k8s.io/client-go/kubernetes"

	"github.com/kubecube-io/kubecube/pkg/apis"

	"k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/kubecube-io/kubecube/pkg/utils/exit"

	"github.com/kubecube-io/kubecube/pkg/clog"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	hnc "sigs.k8s.io/multi-tenancy/incubator/hnc/api/v1alpha2"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	// cache for all k8s and crd resource
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(apis.AddToScheme(scheme))

	utilruntime.Must(hnc.AddToScheme(scheme))

	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
}

// Client retrieves k8s resource with cache or not
type Client interface {
	Cache() cache.Cache
	Direct() client.Client
	Metrics() versioned.Interface
	ClientSet() kubernetes.Interface
}

type InternalClient struct {
	client client.Client
	cache  cache.Cache

	rawClientSet kubernetes.Interface
	metrics      versioned.Interface
}

// NewClientFor generate client by config
func NewClientFor(cfg *rest.Config, stopCh chan struct{}) Client {
	var err error
	c := new(InternalClient)

	c.client, err = client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		clog.Error("problem new k8s client: %v", err)
	}

	c.cache, err = cache.New(cfg, cache.Options{Scheme: scheme})
	if err != nil {
		clog.Error("problem new k8s cache: %v", err)
	}

	c.metrics, err = versioned.NewForConfig(cfg)
	if err != nil {
		clog.Error("problem new metrics client: %v", err)
	}

	c.rawClientSet, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		clog.Error("problem new raw k8s clientSet: %v", err)
	}

	ctx := exit.SetupCtxWithStop(context.Background(), stopCh)

	go func() {
		err := c.cache.Start(ctx)
		if err != nil {
			clog.Error("problem start cache: %v", err)
		}
	}()
	c.cache.WaitForCacheSync(ctx)

	return c
}

func (c *InternalClient) Cache() cache.Cache {
	return c.cache
}

func (c *InternalClient) Direct() client.Client {
	return c.client
}

func (c *InternalClient) Metrics() versioned.Interface {
	return c.metrics
}

func (c *InternalClient) ClientSet() kubernetes.Interface {
	return c.rawClientSet
}

// WithSchemes allow add extensions scheme to client
func WithSchemes(fns ...func(s *runtime.Scheme) error) {
	for _, fn := range fns {
		utilruntime.Must(fn(scheme))
	}
}
