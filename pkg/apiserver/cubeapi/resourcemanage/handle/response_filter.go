package resourcemanage

import (
	"github.com/kubecube-io/kubecube/pkg/utils/filter"
	"net/http"
)

type ResponseFilter struct {
	Condition        *filter.Condition
	ConverterContext *filter.ConverterContext
}

func (f *ResponseFilter) filterResponse(r *http.Response) error {
	return filter.NewFilter(f.ConverterContext).ModifyResponse(r, f.Condition)
}
