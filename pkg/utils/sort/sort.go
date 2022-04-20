package sort

import "strings"

// ParseSort sortName=creationTimestamp, sortOrder=asc
func ParseSort(name string, order string, sFunc string) (sortName, sortOrder, sortFunc string) {
	sortName = "metadata.name"
	sortOrder = "asc"
	sortFunc = "string"

	if name == "" {
		return
	}
	sortName = name

	if strings.EqualFold(order, "desc") {
		sortOrder = "desc"
	}

	if sFunc != "" {
		sortFunc = sFunc
	}

	return
}
