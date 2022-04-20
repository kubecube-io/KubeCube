package selector

import "strings"

// ParseSelector exact query：selector=key1=value1,key2=value2,key3=value3
// fuzzy query：selector=key1~value1,key2~value2,key3~value3
// support mixed query：selector=key1~value1,key2=value2,key3=value3
func ParseSelector(selectorStr string) (exact, fuzzy map[string]string) {
	if selectorStr == "" {
		return nil, nil
	}

	exact = make(map[string]string, 0)
	fuzzy = make(map[string]string, 0)

	labels := strings.Split(selectorStr, ",")
	for _, label := range labels {
		if i := strings.IndexAny(label, "~="); i > 0 {
			if label[i] == '=' {
				exact[label[:i]] = label[i+1:]
			} else {
				fuzzy[label[:i]] = label[i+1:]
			}
		}
	}

	return
}