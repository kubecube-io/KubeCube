package filter

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Handler interface {
	setNext(handler Handler)
	handle(items []unstructured.Unstructured) ([]unstructured.Unstructured, error)
	next(items []unstructured.Unstructured) ([]unstructured.Unstructured, error)
}
