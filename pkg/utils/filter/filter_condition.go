package filter

import "k8s.io/apimachinery/pkg/util/sets"

type Condition struct {
	Exact     map[string]sets.String
	Fuzzy     map[string][]string
	Limit     int
	Offset    int
	SortName  string
	SortOrder string
	SortFunc  string
}

func PageFilterOption(limit int, offset int) *Condition {
	return &Condition{Limit: limit, Offset: offset}
}
