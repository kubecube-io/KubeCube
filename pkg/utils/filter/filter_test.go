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
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	v1 "github.com/kubecube-io/kubecube/pkg/apis/cluster/v1"
)

var _ = ginkgo.Describe("Filter", func() {
	var list = func() *v1.ClusterList {
		clusterList := &v1.ClusterList{
			Items: make([]v1.Cluster, 0),
		}
		clusterList.Kind = "List"
		clusterList.APIVersion = "v1"
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
			clusterList.Items = append(clusterList.Items, l2)
			// must sleep 1 second here to make create timestamp different
			time.Sleep(1 * time.Second)
		}
		return clusterList
	}()

	ginkgo.It("TestModifyResponses", func() {
		listJson, _ := json.Marshal(list.DeepCopy())
		r := http.Response{}
		buf := bytes.NewBufferString(string(listJson))
		r.Body = io.NopCloser(buf)
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

		body, err := io.ReadAll(r.Body)
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

	ginkgo.It("TestFilterResult", func() {
		internalList := list.DeepCopy()
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

		_, err := GetEmptyFilter().FilterObjectList(internalList, condition)
		Expect(err).To(BeNil())

		for i, item := range internalList.Items {
			Expect("a-name" + strconv.Itoa(i+5)).To(Equal(item.Name))
		}
	})

	ginkgo.It("TestExactMatch", func() {
		internalList := list.DeepCopy()
		// create condition
		exact := make(map[string]sets.Set[string])
		exact["metadata.name"] = sets.New[string]("a-name2")
		condition := &Condition{
			Exact: exact,
		}
		_, err := GetEmptyFilter().FilterObjectList(internalList, condition)
		Expect(err).To(BeNil())
		Expect(1).To(Equal(len(internalList.Items)))
		Expect("a-name2").To(Equal(internalList.Items[0].Name))
	})

	ginkgo.It("TestArrayExactMatch", func() {
		internalList := list.DeepCopy()
		// create condition
		exact := make(map[string]sets.Set[string])
		exact["metadata.annotations.kubecube.test.io/app"] = sets.New[string]("a-name2", "b-name2")
		condition := &Condition{
			Exact: exact,
		}
		_, err := GetEmptyFilter().FilterObjectList(internalList, condition)
		Expect(err).To(BeNil())
		Expect(1).To(Equal(len(internalList.Items)))
		Expect("a-name2").To(Equal(internalList.Items[0].Name))
	})

	ginkgo.It("TestExactArrayMatch", func() {
		internalList := list.DeepCopy()
		// create condition
		exact := make(map[string]sets.Set[string])
		exact["metadata.annotations.kubecube.test.io/app"] = sets.New[string]("a-name1", "a-name2")
		condition := &Condition{
			Exact: exact,
		}
		_, err := GetEmptyFilter().FilterObjectList(internalList, condition)
		Expect(err).To(BeNil())
		Expect(2).To(Equal(len(internalList.Items)))
		Expect("a-name1").To(Equal(internalList.Items[0].Name))
		Expect("a-name2").To(Equal(internalList.Items[1].Name))
	})

	ginkgo.It("TestFuzzyMatch", func() {
		internalList := list.DeepCopy()
		// create condition
		fuzzy := make(map[string][]string)
		fuzzy["metadata.name"] = []string{"b-name"}
		condition := &Condition{
			Fuzzy: fuzzy,
		}
		_, err := GetEmptyFilter().FilterObjectList(internalList, condition)
		Expect(err).To(BeNil())
		for i, item := range internalList.Items {
			Expect("b-name" + strconv.Itoa(i+11)).To(Equal(item.Name))
		}
	})

	ginkgo.It("TestArrayFuzzyMatch", func() {
		internalList := list.DeepCopy()
		// create condition
		fuzzy := make(map[string][]string)
		fuzzy["metadata.annotations.kubecube.test.io/app"] = []string{"-name2"}
		condition := &Condition{
			Fuzzy: fuzzy,
		}
		_, err := GetEmptyFilter().FilterObjectList(internalList, condition)
		Expect(err).To(BeNil())
		Expect(1).To(Equal(len(internalList.Items)))
		Expect("a-name2").To(Equal(internalList.Items[0].Name))
	})

	ginkgo.It("TestFuzzyArrayMatch", func() {
		internalList := list.DeepCopy()
		// create condition
		fuzzy := make(map[string][]string)
		fuzzy["metadata.name"] = []string{"-name11", "-name12"}

		condition := &Condition{
			Fuzzy: fuzzy,
		}
		_, err := GetEmptyFilter().FilterObjectList(internalList, condition)
		Expect(err).To(BeNil())
		Expect(2).To(Equal(len(internalList.Items)))
		Expect("b-name11").To(Equal(internalList.Items[0].Name))
		Expect("b-name12").To(Equal(internalList.Items[1].Name))
	})

	ginkgo.It("TestGetDeepValue", func() {
		listJson, _ := json.Marshal(list.DeepCopy())
		var result map[string]interface{}
		err := json.Unmarshal(listJson, &result)
		Expect(err).To(BeNil())

		items := result["items"].([]interface{})
		value, err := GetDeepValue(items[0], "metadata.labels.hello")
		Expect(err).To(BeNil())
		Expect(1).To(Equal(len(value)))
		Expect("world").To(Equal(value[0]))
		_, err = GetDeepValue(items[0], "metadata.labels.hello1")
		Expect(err.Error()).To(Equal("field hello1 not exist"))
	})

})
