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
	"k8s.io/apimachinery/pkg/util/sets"
)

var _ = Describe("Filter", func() {
	var (
		list = make(map[string]interface{})
	)

	BeforeEach(func() {
		list["kind"] = "List"
		var l2s []map[string]interface{}
		for i := 0; i < 20; i++ {
			l2 := make(map[string]interface{})

			l3 := make(map[string]interface{})
			if i <= 10 {
				l3["name"] = "a-name" + strconv.Itoa(i)
			} else {
				l3["name"] = "b-name" + strconv.Itoa(i)
			}
			l3["index"] = i
			l3["creationTimestamp"] = time.Now()
			// labels
			l4 := make(map[string]interface{})
			l4["hello"] = "world"
			l4["number"] = 5
			l3["labels"] = l4
			annotations := make(map[string]interface{})
			annotations["kubecube.test.io/index"] = []string{strconv.Itoa(i)}
			annotations["kubecube.test.io/app"] = []string{"a-name" + strconv.Itoa(i), "b-name" + strconv.Itoa(i)}
			l3["annotations"] = annotations
			l2["metadata"] = l3
			l2s = append(l2s, l2)
		}
		list["items"] = l2s
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

		filter := Filter{
			Exact:        nil,
			Fuzzy:        fuzzy,
			Limit:        5,
			Offset:       5,
			SortName:     "metadata.creationTimestamp",
			SortOrder:    "asc",
			SortFunc:     "time",
			EnableFilter: true,
		}

		filter.ModifyResponse(&r)

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
		// create http.response
		listJson, _ := json.Marshal(list)

		// create condition
		fuzzy := make(map[string][]string)
		fuzzy["metadata.name"] = []string{"a-name"}

		filter := Filter{
			Exact:        nil,
			Fuzzy:        fuzzy,
			Limit:        5,
			Offset:       5,
			SortName:     "metadata.creationTimestamp",
			SortOrder:    "asc",
			SortFunc:     "time",
			EnableFilter: true,
		}

		resultJson := filter.FilterResult(listJson)
		var result map[string]interface{}
		err := json.Unmarshal(resultJson, &result)
		Expect(err).To(BeNil())

		items, ok := result["items"].([]interface{})
		Expect(ok).To(Equal(true))

		for i, item := range items {
			Expect("a-name" + strconv.Itoa(i+5)).To(Equal(item.(map[string]interface{})["metadata"].(map[string]interface{})["name"].(string)))
		}
	})

	It("TestExactMatch", func() {
		// create http.response
		listJson, _ := json.Marshal(list)
		var result map[string]interface{}
		err := json.Unmarshal(listJson, &result)
		Expect(err).To(BeNil())

		items := result["items"].([]interface{})
		// create condition
		exact := make(map[string]sets.String)
		exact["metadata.name"] = sets.NewString("a-name2")

		filter := Filter{
			Exact:     exact,
			Fuzzy:     nil,
			Limit:     5,
			Offset:    0,
			SortName:  "metadata.creationTimestamp",
			SortOrder: "asc",
			SortFunc:  "time",
		}

		items = filter.exactMatch(items)

		Expect(1).To(Equal(len(items)))
		Expect("a-name2").To(Equal(items[0].(map[string]interface{})["metadata"].(map[string]interface{})["name"].(string)))
	})

	It("TestArrayExactMatch", func() {
		listJson, _ := json.Marshal(list)
		var result map[string]interface{}
		err := json.Unmarshal(listJson, &result)
		Expect(err).To(BeNil())

		items := result["items"].([]interface{})
		// create condition
		exact := make(map[string]sets.String)
		exact["metadata.annotations.kubecube.test.io/app"] = sets.NewString("a-name2")
		exact["metadata.annotations.kubecube.test.io/app"] = sets.NewString("b-name2")

		filter := Filter{
			Exact:     exact,
			Fuzzy:     nil,
			Limit:     5,
			Offset:    0,
			SortName:  "metadata.creationTimestamp",
			SortOrder: "asc",
			SortFunc:  "time",
		}

		items = filter.exactMatch(items)
		Expect(1).To(Equal(len(items)))
		Expect("a-name2").To(Equal(items[0].(map[string]interface{})["metadata"].(map[string]interface{})["name"].(string)))
	})

	It("TestExactArrayMatch", func() {
		listJson, _ := json.Marshal(list)
		var result map[string]interface{}
		err := json.Unmarshal(listJson, &result)
		Expect(err).To(BeNil())

		items := result["items"].([]interface{})
		// create condition
		exact := make(map[string]sets.String)
		exact["metadata.annotations.kubecube.test.io/app"] = sets.NewString("a-name1", "a-name2")

		filter := Filter{
			Exact:     exact,
			Fuzzy:     nil,
			Limit:     5,
			Offset:    0,
			SortName:  "metadata.creationTimestamp",
			SortOrder: "asc",
			SortFunc:  "time",
		}

		items = filter.exactMatch(items)
		Expect(2).To(Equal(len(items)))
		Expect("a-name1").To(Equal(items[0].(map[string]interface{})["metadata"].(map[string]interface{})["name"].(string)))
		Expect("a-name2").To(Equal(items[1].(map[string]interface{})["metadata"].(map[string]interface{})["name"].(string)))
	})

	It("TestFuzzyMatch", func() {
		listJson, _ := json.Marshal(list)
		var result map[string]interface{}
		err := json.Unmarshal(listJson, &result)
		Expect(err).To(BeNil())

		items := result["items"].([]interface{})
		// create condition
		fuzzy := make(map[string][]string)
		fuzzy["metadata.name"] = []string{"b-name"}

		filter := Filter{
			Exact:     nil,
			Fuzzy:     fuzzy,
			Limit:     5,
			Offset:    0,
			SortName:  "metadata.creationTimestamp",
			SortOrder: "asc",
			SortFunc:  "time",
		}

		items = filter.fuzzyMatch(items)

		for i, item := range items {
			Expect("b-name" + strconv.Itoa(i+11)).To(Equal(item.(map[string]interface{})["metadata"].(map[string]interface{})["name"].(string)))
		}
	})

	It("TestArrayFuzzyMatch", func() {
		listJson, _ := json.Marshal(list)
		var result map[string]interface{}
		err := json.Unmarshal(listJson, &result)
		Expect(err).To(BeNil())

		items := result["items"].([]interface{})
		// create condition
		fuzzy := make(map[string][]string)
		fuzzy["metadata.annotations.kubecube.test.io/app"] = []string{"-name2"}

		filter := Filter{
			Exact:     nil,
			Fuzzy:     fuzzy,
			Limit:     5,
			Offset:    0,
			SortName:  "metadata.creationTimestamp",
			SortOrder: "asc",
			SortFunc:  "time",
		}

		items = filter.fuzzyMatch(items)
		Expect(1).To(Equal(len(items)))
		Expect("a-name2").To(Equal(items[0].(map[string]interface{})["metadata"].(map[string]interface{})["name"].(string)))
	})

	It("TestFuzzyArrayMatch", func() {
		listJson, _ := json.Marshal(list)
		var result map[string]interface{}
		err := json.Unmarshal(listJson, &result)
		Expect(err).To(BeNil())

		items := result["items"].([]interface{})
		// create condition
		fuzzy := make(map[string][]string)
		fuzzy["metadata.name"] = []string{"-name11", "-name12"}

		filter := Filter{
			Exact:     nil,
			Fuzzy:     fuzzy,
			Limit:     5,
			Offset:    0,
			SortName:  "metadata.creationTimestamp",
			SortOrder: "asc",
			SortFunc:  "time",
		}

		items = filter.fuzzyMatch(items)
		Expect(2).To(Equal(len(items)))
		Expect("b-name11").To(Equal(items[0].(map[string]interface{})["metadata"].(map[string]interface{})["name"].(string)))
		Expect("b-name12").To(Equal(items[1].(map[string]interface{})["metadata"].(map[string]interface{})["name"].(string)))
	})

	It("TestSort", func() {
		listJson, _ := json.Marshal(list)
		var result map[string]interface{}
		err := json.Unmarshal(listJson, &result)
		Expect(err).To(BeNil())

		items := result["items"].([]interface{})
		// create condition
		exact := make(map[string]sets.String)
		exact["metadata.name"] = sets.NewString("a-name2")
		fuzzy := make(map[string][]string)
		fuzzy["metadata.name"] = []string{"b-name"}

		filter := Filter{
			Exact:     exact,
			Fuzzy:     fuzzy,
			Limit:     20,
			Offset:    0,
			SortName:  "metadata.index",
			SortOrder: "desc",
			SortFunc:  "number",
		}

		items = filter.sort(items)

		for i, item := range items {
			if i < 9 {
				Expect("b-name" + strconv.Itoa(19-i)).To(Equal(item.(map[string]interface{})["metadata"].(map[string]interface{})["name"].(string)))
			} else {
				Expect("a-name" + strconv.Itoa(19-i)).To(Equal(item.(map[string]interface{})["metadata"].(map[string]interface{})["name"].(string)))
			}

		}
	})

	It("TestPage", func() {
		listJson, _ := json.Marshal(list)
		var result map[string]interface{}
		err := json.Unmarshal(listJson, &result)
		Expect(err).To(BeNil())

		items := result["items"].([]interface{})
		// create condition
		exact := make(map[string]sets.String)
		exact["metadata.name"] = sets.NewString("a-name2")
		fuzzy := make(map[string][]string)
		fuzzy["metadata.name"] = []string{"b-name"}

		filter := Filter{
			Exact:     exact,
			Fuzzy:     fuzzy,
			Limit:     2,
			Offset:    2,
			SortName:  "metadata.index",
			SortOrder: "desc",
			SortFunc:  "number",
		}

		items = filter.page(items)
		Expect(2).To(Equal(len(items)))
		Expect("a-name2").To(Equal(items[0].(map[string]interface{})["metadata"].(map[string]interface{})["name"].(string)))
		Expect("a-name3").To(Equal(items[1].(map[string]interface{})["metadata"].(map[string]interface{})["name"].(string)))
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

	It("TestGetDeepFloat64", func() {
		listJson, _ := json.Marshal(list)
		var result map[string]interface{}
		err := json.Unmarshal(listJson, &result)
		Expect(err).To(BeNil())

		items := result["items"].([]interface{})
		number := GetDeepFloat64(items[0], "metadata.labels.number")
		Expect(float64(5)).To(Equal(number))
		number = GetDeepFloat64(items[0], "metadata.labels.number1")
		Expect(float64(0)).To(Equal(number))
	})
})
