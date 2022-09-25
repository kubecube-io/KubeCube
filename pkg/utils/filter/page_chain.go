package filter

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

func PageFilterChain(limit int, offset int) *PageParam {
	return &PageParam{
		limit:  limit,
		offset: offset,
		total:  new(int),
	}
}

type PageParam struct {
	limit   int
	offset  int
	total   *int
	handler Handler
}

func (param *PageParam) setNext(handler Handler) {
	param.handler = handler
}
func (param *PageParam) handle(items []unstructured.Unstructured) ([]unstructured.Unstructured, error) {
	*param.total = len(items)
	if len(items) == 0 {
		return param.next(items)
	}
	size := len(items)
	if param.offset >= size {
		return items[0:0], nil
	}
	end := param.offset + param.limit
	if end > size {
		end = size
	}
	return param.next(items[param.offset:end])
}

func (param *PageParam) next(items []unstructured.Unstructured) ([]unstructured.Unstructured, error) {
	if param.handler == nil {
		return items, nil
	}
	return param.handler.handle(items)
}
