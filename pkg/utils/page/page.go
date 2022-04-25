/*
Copyright 2022 KubeCube Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
