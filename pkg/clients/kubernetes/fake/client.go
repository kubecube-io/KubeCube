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

package fake

import (
	k8s "github.com/kubecube-io/kubecube/pkg/clients/kubernetes"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/metrics/pkg/client/clientset/versioned"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache/informertest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cacheFake "github.com/kubecube-io/kubecube/pkg/clients/kubernetes/fake/cache"
	clientSetFake "k8s.io/client-go/kubernetes/fake"
	metricsFake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
	clientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ k8s.Client = &FakerClient{}

// FakerClient implement kubernetes.Client
type FakerClient struct {
	client client.Client
	cache  cache.Cache

	rawClientSet kubernetes.Interface
	metrics      versioned.Interface
}

// Options used to generate fake client
type Options struct {
	Scheme               *runtime.Scheme
	Objs                 []client.Object
	Lists                []client.ObjectList
	ClientRuntimeObjs    []runtime.Object
	ClientSetRuntimeObjs []runtime.Object
	MetricsRuntimeObjs   []runtime.Object
}

// NewFakeClientsFor new fake client by customize
func NewFakeClientsFor(fn func(c *FakerClient)) k8s.Client {
	c := new(FakerClient)
	fn(c)
	return c
}

// NewFakeClients new fake client by given options
func NewFakeClients(opts *Options) k8s.Client {
	c := new(FakerClient)

	cli := clientFake.NewClientBuilder().WithScheme(opts.Scheme).WithObjects(opts.Objs...).WithRuntimeObjects(opts.ClientRuntimeObjs...).WithLists(opts.Lists...).Build()
	c.client = cli
	c.rawClientSet = clientSetFake.NewSimpleClientset(opts.ClientSetRuntimeObjs...)
	c.metrics = metricsFake.NewSimpleClientset(opts.MetricsRuntimeObjs...)
	c.cache = &cacheFake.FakeClient{
		Client: cli,
		Cache:  &informertest.FakeInformers{Scheme: opts.Scheme},
	}

	return c

}

func (c *FakerClient) Cache() cache.Cache {
	return c.cache
}

func (c *FakerClient) Direct() client.Client {
	return c.client
}

func (c *FakerClient) Metrics() versioned.Interface {
	return c.metrics
}

func (c *FakerClient) ClientSet() kubernetes.Interface {
	return c.rawClientSet
}
