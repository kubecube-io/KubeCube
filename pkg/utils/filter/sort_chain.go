package filter

import (
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubecube-io/kubecube/pkg/clog"
)

func SortFilterChain(sortName string, sortOrder string, sortFunc string) *SortParam {
	return &SortParam{
		sortName:  sortName,
		sortOrder: sortOrder,
		sortFunc:  sortFunc,
	}
}

type SortParam struct {
	sortName  string
	sortOrder string
	sortFunc  string
	handler   Handler
}

func (param *SortParam) setNext(handler Handler) {
	param.handler = handler
}
func (param *SortParam) handle(items []unstructured.Unstructured) ([]unstructured.Unstructured, error) {
	if len(items) == 0 {
		return param.next(items)
	}
	sort.Slice(items, func(i, j int) bool {
		getStringFunc := func(items []unstructured.Unstructured, i int, j int) (string, string, error) {
			si, err := GetDeepValue(items[i], param.sortName)
			if err != nil {
				clog.Error("get sort value error, err: %+v", err)
				return "", "", err
			}
			if len(si) > 1 {
				clog.Error("not support array value, val: %+v", si)
				return "", "", err
			}
			sj, err := GetDeepValue(items[j], param.sortName)
			if err != nil {
				clog.Error("get sort value error, err: %+v", err)
				return "", "", err
			}
			if len(sj) > 1 {
				clog.Error("not support array value, val: %+v", sj)
				return "", "", err
			}
			before := si[0]
			after := sj[0]
			return before, after, nil
		}
		switch param.sortFunc {
		case "string":
			before, after, err := getStringFunc(items, i, j)
			if err != nil {
				return false
			}
			if param.sortFunc == "asc" {
				return strings.Compare(before, after) < 0
			} else {
				return strings.Compare(before, after) > 0
			}
		case "time":
			before, after, err := getStringFunc(items, i, j)
			if err != nil {
				return false
			}
			ti, err := time.Parse("2006-01-02T15:04:05Z", before)
			if err != nil {
				return false
			}
			tj, err := time.Parse("2006-01-02T15:04:05Z", after)
			if err != nil {
				return false
			} else if param.sortOrder == "asc" {
				return ti.Before(tj)
			} else {
				return ti.After(tj)
			}
		case "number":
			ni := GetDeepFloat64(items[i], param.sortName)
			nj := GetDeepFloat64(items[j], param.sortName)
			if param.sortOrder == "asc" {
				return ni < nj
			} else if param.sortOrder == "desc" {
				return ni > nj
			} else {
				return ni < nj
			}
		default:
			before, after, err := getStringFunc(items, i, j)
			if err != nil {
				return false
			}
			if param.sortOrder == "asc" {
				return strings.Compare(before, after) < 0
			} else {
				return strings.Compare(before, after) > 0
			}
		}
	})
	return param.next(items)
}

func (param *SortParam) next(items []unstructured.Unstructured) ([]unstructured.Unstructured, error) {
	if param.handler == nil {
		return items, nil
	}
	return param.handler.handle(items)
}
