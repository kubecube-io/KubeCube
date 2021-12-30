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
package international

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/gogf/gf/v2/i18n/gi18n"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
)

type Gi18nManagers struct {
	Managers map[string]*gi18n.Manager
	lock     sync.RWMutex
}

type Config struct {
	Languages map[string]string
}

func InitGi18nManagers() (*Gi18nManagers, error) {
	// read kubecube-language-config
	kClient := clients.Interface().Kubernetes(constants.PivotCluster).Direct()
	if kClient == nil {
		return nil, errors.New("get pivot cluster client is nil")
	}
	cm := &v1.ConfigMap{}
	err := kClient.Get(context.Background(), client.ObjectKey{Name: "kubecube-language-config", Namespace: constants.CubeNamespace}, cm)
	if err != nil {
		clog.Error("get configmap kubecube-language-config from K8s err: %v", err)
		return nil, err
	}

	var languages []string
	languages = strings.Split(cm.Data["languages"], ",")
	m := &Gi18nManagers{
		Managers: make(map[string]*gi18n.Manager),
	}
	for _, l := range languages {
		inst := gi18n.Instance(l)
		inst.SetLanguage(l)
		m.Managers[l] = inst
	}
	return m, nil
}

func (g *Gi18nManagers) GetInstants(language string) *gi18n.Manager {
	g.lock.RLock()
	defer g.lock.RUnlock()
	return g.Managers[language]
}

func (g *Gi18nManagers) Translate(ctx context.Context, language string, content string) string {
	g.lock.RLock()
	m := g.Managers[language]
	g.lock.RUnlock()
	return m.Translate(ctx, content)
}
