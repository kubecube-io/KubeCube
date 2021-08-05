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

package hotplug

import (
	"bytes"
	"context"
	"encoding/json"
	"text/template"

	hotplugv1 "github.com/kubecube-io/kubecube/pkg/apis/hotplug/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/warden/utils"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8syaml "sigs.k8s.io/yaml"
)

// translate string to json
func YamlStringToJson(yamlStr string) (map[string]interface{}, error) {
	b, err := k8syaml.YAMLToJSON([]byte(yamlStr))
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	err = json.Unmarshal(b, &m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// merge json
func MergeJson(a, b map[string]interface{}) map[string]interface{} {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	for key, bVal := range b {
		if aVal, ok := a[key]; ok {
			aMap, ok1 := aVal.(map[string]interface{})
			bMap, ok2 := bVal.(map[string]interface{})
			if ok1 && ok2 {
				bVal = MergeJson(aMap, bMap)
			}
		}
		if bVal != nil {
			a[key] = bVal
		}
	}
	return a
}

// merge yaml string
func MergeYamlString(ayaml, byaml string) (string, error) {
	ajson, err := YamlStringToJson(ayaml)
	if err != nil {
		return ayaml, err
	}
	bjson, err := YamlStringToJson(byaml)
	if err != nil {
		return ayaml, err
	}
	ret := MergeJson(ajson, bjson)
	retb, err := k8syaml.Marshal(ret)
	if err != nil {
		return ayaml, err
	}
	return string(retb), nil
}

// merge hotplug config
func MergeHotplug(commonConfig, clusterConfig hotplugv1.Hotplug) hotplugv1.Hotplug {
	clusterComponentMap := make(map[string]hotplugv1.ComponentConfig)
	for _, item := range clusterConfig.Spec.Component {
		clusterComponentMap[item.Name] = item
	}
	var componentArr []hotplugv1.ComponentConfig
	for _, i := range commonConfig.Spec.Component {
		j, ok := clusterComponentMap[i.Name]
		if !ok {
			i.Env = convertYaml(i.Env)
			componentArr = append(componentArr, i)
			continue
		}
		if j.Namespace != "" {
			i.Namespace = j.Namespace
		}
		if j.Status != "" {
			i.Status = j.Status
		}
		if j.PkgName != "" {
			i.PkgName = j.PkgName
		}
		if j.Env != "" {
			env, err := MergeYamlString(i.Env, j.Env)
			if err != nil {
				clog.Info("can not merge cluster config env to common config env, %s", i.Name)
				continue
			}
			i.Env = env
		}
		i.Env = convertYaml(i.Env)
		componentArr = append(componentArr, i)
	}
	commonConfig.Spec.Component = componentArr
	return commonConfig
}

// judge json equal
func JudgeJsonEqual(a interface{}, b interface{}) bool {
	am, aok := a.(map[string]interface{})
	bm, bok := b.(map[string]interface{})
	if !aok || !bok {
		aj, err := json.Marshal(a)
		if err != nil {
			return false
		}
		bj, err := json.Marshal(b)
		if err != nil {
			return false
		}
		return string(aj) == string(bj)
	}
	for k, av := range am {
		if bv, ok := bm[k]; ok {
			ret := JudgeJsonEqual(av, bv)
			if !ret {
				return false
			}
		} else {
			return false
		}
	}
	for k, bv := range bm {
		if av, ok := am[k]; ok {
			ret := JudgeJsonEqual(bv, av)
			if !ret {
				return false
			}
		} else {
			return false
		}
	}
	return true
}

// replace {{.cluster}} to real clusterName
func convertYaml(yamlStr string) string {
	if yamlStr == "" {
		return ""
	}
	templateParams := make(map[string]string)
	templateParams["cluster"] = utils.Cluster
	tmpl, err := template.New("env").Parse(yamlStr)
	if err != nil {
		clog.Info("can not parse env %v, %v", yamlStr, err)
		return yamlStr
	}
	var b1 bytes.Buffer
	err = tmpl.Execute(&b1, templateParams)
	if err != nil {
		clog.Info("can not parse env and set clusterName, %v, %v", yamlStr, err)
		return yamlStr
	}
	return b1.String()
}

// update feature configmap
func updateConfigMap(ctx context.Context, cli client.Client, result *hotplugv1.DeployResult) {
	cm := corev1.ConfigMap{}
	err := cli.Get(ctx, types.NamespacedName{Namespace: utils.Namespace, Name: utils.FeatureConfigMap}, &cm)
	if err != nil {
		clog.Error("can not find configmap kubecube-system/kubecube-feature-config, %v", err)
		return
	}
	if cm.Data == nil {
		m := make(map[string]string)
		m[result.Name] = result.Status
		cm.Data = m
	} else {
		cm.Data[result.Name] = result.Status
	}
	err = cli.Update(ctx, &cm)
	if err != nil {
		clog.Error("can not find configmap kubecube-system/kubecube-feature-config, %v", err)
		return
	}
}
