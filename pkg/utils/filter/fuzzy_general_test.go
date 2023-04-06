package filter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFuzzyMatch(t *testing.T) {
	// case1: not a slice type
	_, err := FuzzyMatch("abc", FuzzyMatchCondition{})
	assert.Error(t, err)

	// case2: not suitable elems
	slice := []string{"aaa", "bbb", "ccc"}
	results, err := FuzzyMatch(slice, FuzzyMatchCondition{SearchStr: "ddd", FieldExtractor: func(obj interface{}) string {
		return obj.(string)
	}})
	assert.NoError(t, err)
	assert.Empty(t, results)

	// case3: had suitable a elem
	slice = []string{"aaa", "bbb", "ccc"}
	results, err = FuzzyMatch(slice, FuzzyMatchCondition{SearchStr: "bb", FieldExtractor: func(obj interface{}) string {
		return obj.(string)
	}})
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{"bbb"}, results)

	// case4: had many suitable elems
	type Person struct {
		Name string
		Age  int
		Like string
	}
	slice2 := []Person{
		{"Alice", 25, "jump"},
		{"Bob", 30, "rap"},
		{"Charlie", 35, "ball"},
		{"David", 40, "jump"},
	}
	results, err = FuzzyMatch(slice2, FuzzyMatchCondition{SearchStr: "i", FieldExtractor: func(obj interface{}) string {
		return obj.(Person).Name
	}}, FuzzyMatchCondition{SearchStr: "jump", FieldExtractor: func(obj interface{}) string {
		return obj.(Person).Like
	}})
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{Person{"Alice", 25, "jump"}, Person{"David", 40, "jump"}}, results)
}
