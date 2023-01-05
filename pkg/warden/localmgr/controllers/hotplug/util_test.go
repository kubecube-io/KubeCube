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

package hotplug_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	hotplugv1 "github.com/kubecube-io/kubecube/pkg/apis/hotplug/v1"
	"github.com/kubecube-io/kubecube/pkg/warden/localmgr/controllers/hotplug"
)

var _ = Describe("Util", func() {

	It("test yanl string to json", func() {
		yamlStr := `
test1:
  test1-1:
   test1-1-1: 111
   test1-1-2: 112
test2:
  test2-1: 21
  test2-2: 22
test3:
  test3-1:
  - test3-1-1: 311
  - test3-1-2: 312
`
		r, err := hotplug.YamlStringToJson(yamlStr)
		Expect(err).To(BeNil())
		Expect(r["test2"].(map[string]interface{})["test2-1"].(float64)).To(Equal(float64(21)))
		rb, err := json.Marshal(r)
		Expect(err).To(BeNil())
		Expect(string(rb)).To(Equal("{\"test1\":{\"test1-1\":{\"test1-1-1\":111,\"test1-1-2\":112}},\"test2\":{\"test2-1\":21,\"test2-2\":22},\"test3\":{\"test3-1\":[{\"test3-1-1\":311},{\"test3-1-2\":312}]}}"))
	})

	It("test mergin json", func() {
		json1 := "{\"a\": {\"aa\": \"aa\"}, \"b\": {\"bb\": \"bb\"}, \"c\": {\"cc\": {\"ccc\": \"ccc1\"}}}"
		json2 := "{\"a\": {\"ab\": \"ab\"}, \"c\": {\"cc\": {\"ccc\": \"ccc2\"}}, \"d\": {\"dd\": \"dd\"}}"
		var m1 map[string]interface{}
		var m2 map[string]interface{}
		err := json.Unmarshal([]byte(json1), &m1)
		Expect(err).To(BeNil())
		err = json.Unmarshal([]byte(json2), &m2)
		Expect(err).To(BeNil())
		m := hotplug.MergeJson(m1, m2)
		b, err := json.Marshal(m)
		Expect(err).To(BeNil())
		Expect(string(b)).To(Equal("{\"a\":{\"aa\":\"aa\",\"ab\":\"ab\"},\"b\":{\"bb\":\"bb\"},\"c\":{\"cc\":{\"ccc\":\"ccc2\"}},\"d\":{\"dd\":\"dd\"}}"))
	})

	It("test mergin yaml", func() {
		yamlStr1 := `
a:
  aa: aa
b:
  bb: bb
c:
  cc:
    ccc: ccc1
`
		yamlStr2 := `
a:
  ab: ab
c:
  cc:
    ccc: ccc2
d:
  dd: dd
`
		yamlStr3 := `a:
  aa: aa
  ab: ab
b:
  bb: bb
c:
  cc:
    ccc: ccc2
d:
  dd: dd
`
		s, err := hotplug.MergeYamlString(yamlStr1, yamlStr2)
		Expect(err).To(BeNil())
		Expect(s).To(Equal(yamlStr3))
	})

	It("test mergin hotplug", func() {
		yamlStr1 := `
a:
  aa: aa
b:
  bb: bb
c:
  cc:
    ccc: ccc1
`
		yamlStr2 := `
a:
  ab: ab
c:
  cc:
    ccc: ccc2
d:
  dd: dd
`
		yamlStr3 := `a:
  aa: aa
  ab: ab
b:
  bb: bb
c:
  cc:
    ccc: ccc2
d:
  dd: dd
`
		c1 := hotplugv1.Hotplug{
			Spec: hotplugv1.HotplugSpec{
				Component: []hotplugv1.ComponentConfig{
					{Name: "a", Namespace: "ns", Status: "enabled", PkgName: "abc", Env: yamlStr1},
					{Name: "b", Namespace: "ns", Status: "enabled", PkgName: "abc", Env: yamlStr1},
				},
			},
		}
		c2 := hotplugv1.Hotplug{
			Spec: hotplugv1.HotplugSpec{
				Component: []hotplugv1.ComponentConfig{
					{Name: "a", Status: "disabled", PkgName: "abc", Env: yamlStr2},
				},
			},
		}
		c := hotplug.MergeHotplug(c1, c2)
		Expect(len(c.Spec.Component)).To(Equal(2))
		for _, c := range c.Spec.Component {
			switch c.Name {
			case "a":
				Expect(c.Status).To(Equal("disabled"))
				Expect(c.Env).To(Equal(yamlStr3))
			case "b":
				Expect(c.Status).To(Equal("enabled"))
				Expect(c.Env).To(Equal(yamlStr1))
			default:
				panic("mergin hotplug fail")
			}
		}
	})

	It("test judge json equal", func() {
		json1 := "{\"a\": {\"aa\": \"aa\"}, \"b\": [{\"bb\": \"bb\"}], \"c\": {\"cc\": {\"ccc\": \"ccc1\"}}}"
		json2 := "{\"a\": {\"ab\": \"ab\"}, \"c\": {\"cc\": {\"ccc\": \"ccc2\"}}, \"d\": {\"dd\": \"dd\"}}"
		json3 := "{\"a\": {\"aa\": \"aa\"}, \"b\": [{\"bb\": \"bb\"}], \"c\": {\"cc\": {\"ccc\": \"ccc1\"}}}"
		var m1 map[string]interface{}
		var m2 map[string]interface{}
		var m3 map[string]interface{}
		err := json.Unmarshal([]byte(json1), &m1)
		Expect(err).To(BeNil())
		err = json.Unmarshal([]byte(json2), &m2)
		Expect(err).To(BeNil())
		err = json.Unmarshal([]byte(json3), &m3)
		Expect(err).To(BeNil())
		b := hotplug.JudgeJsonEqual(m1, m2)
		Expect(b).To(Equal(false))
		b = hotplug.JudgeJsonEqual(m1, m3)
		Expect(b).To(Equal(true))
	})
})
