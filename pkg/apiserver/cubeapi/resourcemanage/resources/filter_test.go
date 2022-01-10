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
package resources

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubecube-io/kubecube/pkg/clients"
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	"github.com/kubecube-io/kubecube/pkg/multicluster/client/fake"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
)

func createList() map[string]interface{} {
	l1 := make(map[string]interface{})
	l1["kind"] = "List"
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

		l2["metadata"] = l3
		l2s = append(l2s, l2)
	}
	l1["items"] = l2s

	return l1
}

func TestModifyResponse(t *testing.T) {
	assert := assert.New(t)

	// create http.response
	list := createList()
	listJson, _ := json.Marshal(list)
	r := http.Response{}
	buf := bytes.NewBufferString(string(listJson))
	r.Body = ioutil.NopCloser(buf)
	r.Header = make(map[string][]string)
	r.Header["Content-Length"] = []string{fmt.Sprint(buf.Len())}

	// create condition
	exact := make(map[string]string)
	//exact["metadata.name"] = "a-name2"
	fuzzy := make(map[string]string)
	fuzzy["metadata.name"] = "a-name"

	filter := Filter{
		Exact:     exact,
		Fuzzy:     fuzzy,
		Limit:     5,
		Offset:    5,
		SortName:  "metadata.creationTimestamp",
		SortOrder: "asc",
		SortFunc:  "time",
	}

	filter.ModifyResponse(&r)

	body, err := ioutil.ReadAll(r.Body)
	assert.Nil(err)
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	assert.Nil(err)

	items, ok := result["items"].([]interface{})
	assert.Equal(true, ok)

	for i, item := range items {
		assert.Equal("a-name"+strconv.Itoa(i+5), item.(map[string]interface{})["metadata"].(map[string]interface{})["name"].(string))
	}
}

func TestFilterResult(t *testing.T) {
	assert := assert.New(t)

	// create http.response
	list := createList()
	listJson, _ := json.Marshal(list)

	// create condition
	exact := make(map[string]string)
	//exact["metadata.name"] = "a-name2"
	fuzzy := make(map[string]string)
	fuzzy["metadata.name"] = "a-name"

	filter := Filter{
		Exact:     exact,
		Fuzzy:     fuzzy,
		Limit:     5,
		Offset:    5,
		SortName:  "metadata.creationTimestamp",
		SortOrder: "asc",
		SortFunc:  "time",
	}

	resultJson := filter.FilterResult(listJson)
	var result map[string]interface{}
	err := json.Unmarshal(resultJson, &result)
	assert.Nil(err)

	items, ok := result["items"].([]interface{})
	assert.Equal(true, ok)

	for i, item := range items {
		assert.Equal("a-name"+strconv.Itoa(i+5), item.(map[string]interface{})["metadata"].(map[string]interface{})["name"].(string))
	}
}

func TestExactMatch(t *testing.T) {
	assert := assert.New(t)

	// create http.response
	list := createList()
	listJson, _ := json.Marshal(list)
	var result map[string]interface{}
	json.Unmarshal(listJson, &result)

	items := result["items"].([]interface{})

	// create condition
	exact := make(map[string]string)
	exact["metadata.name"] = "a-name2"
	fuzzy := make(map[string]string)
	fuzzy["metadata.name"] = "a-name"

	filter := Filter{
		Exact:     exact,
		Fuzzy:     fuzzy,
		Limit:     5,
		Offset:    0,
		SortName:  "metadata.creationTimestamp",
		SortOrder: "asc",
		SortFunc:  "time",
	}

	items = filter.exactMatch(items)

	assert.Equal("a-name2", items[0].(map[string]interface{})["metadata"].(map[string]interface{})["name"].(string))
}

func TestFuzzyMatch(t *testing.T) {
	assert := assert.New(t)

	// create http.response
	list := createList()
	listJson, _ := json.Marshal(list)
	var result map[string]interface{}
	json.Unmarshal(listJson, &result)

	items := result["items"].([]interface{})

	// create condition
	exact := make(map[string]string)
	exact["metadata.name"] = "a-name2"
	fuzzy := make(map[string]string)
	fuzzy["metadata.name"] = "b-name"

	filter := Filter{
		Exact:     exact,
		Fuzzy:     fuzzy,
		Limit:     5,
		Offset:    0,
		SortName:  "metadata.creationTimestamp",
		SortOrder: "asc",
		SortFunc:  "time",
	}

	items = filter.fuzzyMatch(items)

	for i, item := range items {
		assert.Equal("b-name"+strconv.Itoa(i+11), item.(map[string]interface{})["metadata"].(map[string]interface{})["name"].(string))
	}
}

func TestSort(t *testing.T) {
	assert := assert.New(t)

	// create http.response
	list := createList()
	listJson, _ := json.Marshal(list)
	var result map[string]interface{}
	json.Unmarshal(listJson, &result)

	items := result["items"].([]interface{})

	// create condition
	exact := make(map[string]string)
	exact["metadata.name"] = "a-name2"
	fuzzy := make(map[string]string)
	fuzzy["metadata.name"] = "b-name"

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
			assert.Equal("b-name"+strconv.Itoa(19-i), item.(map[string]interface{})["metadata"].(map[string]interface{})["name"].(string))
		} else {
			assert.Equal("a-name"+strconv.Itoa(19-i), item.(map[string]interface{})["metadata"].(map[string]interface{})["name"].(string))
		}

	}
}

func TestPage(t *testing.T) {
	assert := assert.New(t)

	// create http.response
	list := createList()
	listJson, _ := json.Marshal(list)
	var result map[string]interface{}
	json.Unmarshal(listJson, &result)

	items := result["items"].([]interface{})

	// create condition
	exact := make(map[string]string)
	exact["metadata.name"] = "a-name2"
	fuzzy := make(map[string]string)
	fuzzy["metadata.name"] = "b-name"

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
	assert.Equal(2, len(items))
	assert.Equal("a-name2", items[0].(map[string]interface{})["metadata"].(map[string]interface{})["name"].(string))
	assert.Equal("a-name3", items[1].(map[string]interface{})["metadata"].(map[string]interface{})["name"].(string))

}

func TestGetDeepValue(t *testing.T) {
	assert := assert.New(t)

	// create http.response
	list := createList()
	listJson, _ := json.Marshal(list)
	var result map[string]interface{}
	json.Unmarshal(listJson, &result)

	items := result["items"].([]interface{})

	value := GetDeepValue(items[0], "metadata.labels.hello")
	assert.Equal("world", value)
	value = GetDeepValue(items[0], "metadata.labels.hello1")
	assert.Equal("", value)
}

func TestGetDeepFloat64(t *testing.T) {
	assert := assert.New(t)

	// create http.response
	list := createList()
	listJson, _ := json.Marshal(list)
	var result map[string]interface{}
	json.Unmarshal(listJson, &result)

	items := result["items"].([]interface{})

	number := GetDeepFloat64(items[0], "metadata.labels.number")
	assert.Equal(float64(5), number)
	number = GetDeepFloat64(items[0], "metadata.labels.number1")
	assert.Equal(float64(0), number)
}

func TestAccessAllow(t *testing.T) {
	scheme := runtime.NewScheme()
	opts := &fake.Options{
		Scheme:               scheme,
		Objs:                 []client.Object{},
		ClientSetRuntimeObjs: []runtime.Object{},
		Lists:                []client.ObjectList{},
	}
	multicluster.InitFakeMultiClusterMgrWithOpts(opts)
	clients.InitCubeClientSetWithOpts(nil)

	assert := assert.New(t)
	access := NewSimpleAccess(constants.LocalCluster, "admin", "namespace-test")
	allow := access.AccessAllow("", "pods", "list")
	assert.Equal(allow, false)
}
