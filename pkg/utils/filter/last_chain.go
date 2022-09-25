package filter

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

type Last struct {
	handler Handler
}

func (last *Last) setNext(handler Handler) {
	last.handler = handler
}
func (last *Last) handle(items []unstructured.Unstructured) ([]unstructured.Unstructured, error) {
	return items, nil
}

func (last *Last) next(_ []unstructured.Unstructured) ([]unstructured.Unstructured, error) {
	return nil, nil
}
