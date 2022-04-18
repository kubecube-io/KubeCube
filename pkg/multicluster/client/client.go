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

package client

import (
	"context"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/metrics/pkg/client/clientset/versioned"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	hnc "sigs.k8s.io/hierarchical-namespaces/api/v1alpha2"

	"github.com/kubecube-io/kubecube/pkg/apis"
	"github.com/kubecube-io/kubecube/pkg/clog"
)

var (
	scheme = initScheme()
)

func initScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	// register for all k8s and crd resource
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	utilruntime.Must(apis.AddToScheme(s))
	utilruntime.Must(hnc.AddToScheme(s))
	utilruntime.Must(apiextensionsv1.AddToScheme(s))
	return s
}

// Client retrieves k8s resource with cache or not
type Client interface {
	Cache() cache.Cache
	Direct() client.Client

	// todo: aggregate to versioned
	Metrics() versioned.Interface
	ClientSet() kubernetes.Interface

	Discovery() discovery.DiscoveryInterface
	RESTMapper() meta.RESTMapper
}

type InternalClient struct {
	client       client.Client
	cache        cache.Cache
	rawClientSet kubernetes.Interface
	metrics      versioned.Interface
	discovery    discovery.DiscoveryInterface

	// restMapper map GroupVersionKinds to Resources
	restMapper meta.RESTMapper
}

// NewClientFor generate client by config
// todo: with options
func NewClientFor(ctx context.Context, cfg *rest.Config) (Client, error) {
	var err error
	c := new(InternalClient)

	c.client, err = client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("new k8s client failed: %v", err)
	}

	c.cache, err = cache.New(cfg, cache.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("new k8s cache failed: %v", err)
	}

	c.metrics, err = versioned.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("new metrics client failed: %v", err)
	}

	c.rawClientSet, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("new raw k8s clientSet failed: %v", err)
	}

	c.discovery, err = discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("new discovery client failed: %v", err)
	}

	c.restMapper, err = apiutil.NewDynamicRESTMapper(cfg)
	if err != nil {
		return nil, fmt.Errorf("new rest mapper failed: %v", err)
	}

	go func() {
		err = c.cache.Start(ctx)
		if err != nil {
			// that should not happened
			clog.Error("start cache failed: %v", err)
		}
	}()
	c.cache.WaitForCacheSync(ctx)

	return c, nil
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

func (c *InternalClient) RESTMapper() meta.RESTMapper {
	return c.restMapper
}

func (c *InternalClient) Discovery() discovery.DiscoveryInterface {
	return c.discovery
}

// WithSchemes allow add extensions scheme to client
func WithSchemes(fns ...func(s *runtime.Scheme) error) {
	for _, fn := range fns {
		utilruntime.Must(fn(scheme))
	}
}
