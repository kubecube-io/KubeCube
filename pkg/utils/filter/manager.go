package filter

var filter *Filter

func init() {
	filter = NewFilter(nil)
}

func GetEmptyFilter() *Filter {
	return filter
}
