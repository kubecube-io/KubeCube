package filter

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type First struct {
	handler Handler
}

func (first *First) setNext(handler Handler) {
	first.handler = handler
}
func (first *First) handle(items []unstructured.Unstructured) ([]unstructured.Unstructured, error) {
	return first.next(items)
}

func (first *First) next(items []unstructured.Unstructured) ([]unstructured.Unstructured, error) {
	if first.handler == nil {
		return items, nil
	}
	return first.handler.handle(items)
}
