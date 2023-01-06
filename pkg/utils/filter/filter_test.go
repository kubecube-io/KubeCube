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

package filter

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	v1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
)

var _ = Describe("Filter", func() {
	var (
		list = v1.ClusterList{
			Items: make([]v1.Cluster, 0),
		}
	)

	BeforeEach(func() {
		list.Kind = "List"
		list.APIVersion = "v1"
		for i := 0; i < 20; i++ {
			l2 := v1.Cluster{}
			l2.Kind = "Cluster"
			l2.APIVersion = "cluster.kubecube.io/v1"
			if i <= 10 {
				l2.SetName("a-name" + strconv.Itoa(i))
			} else {
				l2.SetName("b-name" + strconv.Itoa(i))
			}
			l2.SetCreationTimestamp(metav1.NewTime(time.Now()))
			// labels
			l4 := make(map[string]string)
			l4["hello"] = "world"
			l4["number"] = "5"
			l2.SetLabels(l4)
			annotations := make(map[string]string)
			annotations["kubecube.test.io/index"] = strconv.Itoa(i)
			annotations["kubecube.test.io/app"] = "a-name" + strconv.Itoa(i)
			l2.SetAnnotations(annotations)
			list.Items = append(list.Items, l2)
		}
	})

	AfterEach(func() {
		list = v1.ClusterList{
			Items: make([]v1.Cluster, 0),
		}
	})

	It("TestModifyResponses", func() {
		listJson, _ := json.Marshal(list)
		r := http.Response{}
		buf := bytes.NewBufferString(string(listJson))
		r.Body = ioutil.NopCloser(buf)
		r.Header = make(map[string][]string)
		r.Header["Content-Length"] = []string{fmt.Sprint(buf.Len())}

		// create condition
		fuzzy := make(map[string][]string)
		fuzzy["metadata.name"] = []string{"a-name"}

		condition := &Condition{
			Fuzzy:     fuzzy,
			Limit:     5,
			Offset:    5,
			SortName:  "metadata.creationTimestamp",
			SortFunc:  "time",
			SortOrder: "asc",
		}
		err := GetEmptyFilter().ModifyResponse(&r, condition)
		Expect(err).To(BeNil())

		body, err := ioutil.ReadAll(r.Body)
		Expect(err).To(BeNil())
		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		Expect(err).To(BeNil())

		items, ok := result["items"].([]interface{})
		Expect(ok).To(Equal(true))

		for i, item := range items {
			Expect("a-name" + strconv.Itoa(i+5)).To(Equal(item.(map[string]interface{})["metadata"].(map[string]interface{})["name"].(string)))
		}
	})

	It("TestFilterResult", func() {
		// create condition
		fuzzy := make(map[string][]string)
		fuzzy["metadata.name"] = []string{"a-name"}

		condition := &Condition{
			Fuzzy:     fuzzy,
			Limit:     5,
			Offset:    5,
			SortName:  "metadata.creationTimestamp",
			SortFunc:  "time",
			SortOrder: "asc",
		}

		_, err := GetEmptyFilter().FilterObjectList(&list, condition)
		Expect(err).To(BeNil())

		for i, item := range list.Items {
			Expect("a-name" + strconv.Itoa(i+5)).To(Equal(item.Name))
		}
	})

	It("TestExactMatch", func() {
		// create condition
		exact := make(map[string]sets.String)
		exact["metadata.name"] = sets.NewString("a-name2")
		condition := &Condition{
			Exact: exact,
		}
		_, err := GetEmptyFilter().FilterObjectList(&list, condition)
		Expect(err).To(BeNil())
		Expect(1).To(Equal(len(list.Items)))
		Expect("a-name2").To(Equal(list.Items[0].Name))
	})

	It("TestArrayExactMatch", func() {
		// create condition
		exact := make(map[string]sets.String)
		exact["metadata.annotations.kubecube.test.io/app"] = sets.NewString("a-name2", "b-name2")
		condition := &Condition{
			Exact: exact,
		}
		_, err := GetEmptyFilter().FilterObjectList(&list, condition)
		Expect(err).To(BeNil())
		Expect(1).To(Equal(len(list.Items)))
		Expect("a-name2").To(Equal(list.Items[0].Name))
	})

	It("TestExactArrayMatch", func() {
		// create condition
		exact := make(map[string]sets.String)
		exact["metadata.annotations.kubecube.test.io/app"] = sets.NewString("a-name1", "a-name2")
		condition := &Condition{
			Exact: exact,
		}
		_, err := GetEmptyFilter().FilterObjectList(&list, condition)
		Expect(err).To(BeNil())
		Expect(2).To(Equal(len(list.Items)))
		Expect("a-name1").To(Equal(list.Items[0].Name))
		Expect("a-name2").To(Equal(list.Items[1].Name))
	})

	It("TestFuzzyMatch", func() {
		// create condition
		fuzzy := make(map[string][]string)
		fuzzy["metadata.name"] = []string{"b-name"}
		condition := &Condition{
			Fuzzy: fuzzy,
		}
		_, err := GetEmptyFilter().FilterObjectList(&list, condition)
		Expect(err).To(BeNil())
		for i, item := range list.Items {
			Expect("b-name" + strconv.Itoa(i+11)).To(Equal(item.Name))
		}
	})

	It("TestArrayFuzzyMatch", func() {
		// create condition
		fuzzy := make(map[string][]string)
		fuzzy["metadata.annotations.kubecube.test.io/app"] = []string{"-name2"}
		condition := &Condition{
			Fuzzy: fuzzy,
		}
		_, err := GetEmptyFilter().FilterObjectList(&list, condition)
		Expect(err).To(BeNil())
		Expect(1).To(Equal(len(list.Items)))
		Expect("a-name2").To(Equal(list.Items[0].Name))
	})

	It("TestFuzzyArrayMatch", func() {
		// create condition
		fuzzy := make(map[string][]string)
		fuzzy["metadata.name"] = []string{"-name11", "-name12"}

		condition := &Condition{
			Fuzzy: fuzzy,
		}
		_, err := GetEmptyFilter().FilterObjectList(&list, condition)
		Expect(err).To(BeNil())
		Expect(2).To(Equal(len(list.Items)))
		Expect("b-name11").To(Equal(list.Items[0].Name))
		Expect("b-name12").To(Equal(list.Items[1].Name))
	})

	It("TestSort", func() {
		// create condition
		exact := make(map[string]sets.String)
		exact["metadata.name"] = sets.NewString("a-name2")
		fuzzy := make(map[string][]string)
		fuzzy["metadata.name"] = []string{"b-name"}
		condition := &Condition{
			Exact:     exact,
			Fuzzy:     fuzzy,
			SortName:  "metadata.index",
			SortFunc:  "number",
			SortOrder: "desc",
		}
		_, err := GetEmptyFilter().FilterObjectList(&list, condition)
		Expect(err).To(BeNil())

		for i, item := range list.Items {
			if i < 9 {
				Expect("b-name" + strconv.Itoa(19-i)).To(Equal(item.Name))
			} else {
				Expect("a-name" + strconv.Itoa(19-i)).To(Equal(item.Name))
			}

		}
	})

	It("TestPage", func() {
		// create condition
		condition := &Condition{
			Limit:     2,
			Offset:    2,
			SortName:  "metadata.index",
			SortFunc:  "number",
			SortOrder: "desc",
		}
		_, err := GetEmptyFilter().FilterObjectList(&list, condition)
		Expect(err).To(BeNil())
		Expect(2).To(Equal(len(list.Items)))
		Expect("a-name2").To(Equal(list.Items[0].Name))
		Expect("a-name3").To(Equal(list.Items[1].Name))
	})

	It("TestGetDeepValue", func() {
		listJson, _ := json.Marshal(list)
		var result map[string]interface{}
		err := json.Unmarshal(listJson, &result)
		Expect(err).To(BeNil())

		items := result["items"].([]interface{})
		value, err := GetDeepValue(items[0], "metadata.labels.hello")
		Expect(err).To(BeNil())
		Expect(1).To(Equal(len(value)))
		Expect("world").To(Equal(value[0]))
		_, err = GetDeepValue(items[0], "metadata.labels.hello1")
		Expect(err.Error()).To(Equal("field hello1 not exsit"))
	})

})
