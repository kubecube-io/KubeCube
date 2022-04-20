package page

import "strconv"

// ParsePage page=10,1, means limit=10&page=1, default 10,1
// offset=(page-1)*limit
func ParsePage(pageSize string, pageNum string) (limit, offset int) {
	limit = 10
	offset = 0

	limit, err := strconv.Atoi(pageSize)
	if err != nil {
		limit = 10
	}

	page, err := strconv.Atoi(pageNum)
	if err != nil || page < 1 {
		offset = 0
	} else {
		offset = (page - 1) * limit
	}

	return
}