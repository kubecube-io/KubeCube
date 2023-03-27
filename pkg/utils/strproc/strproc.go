/*
Copyright 2021 KubeCube Authors

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

package strproc

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/kubecube-io/kubecube/pkg/clog"
	"k8s.io/apimachinery/pkg/api/resource"
)

func Str2int(str string) int {
	reg, err := regexp.Compile("[^Z0-9]+")
	if err != nil {
		clog.Error(err.Error())
		return 0
	}

	processedStr := reg.ReplaceAllString(str, "")

	i, err := strconv.Atoi(processedStr)
	if err != nil {
		clog.Error(err.Error())
		return 0
	}

	return i
}

const (
	Ki = "Ki" // 1*1024
	Mi = "Mi" // 1*1024*1024
	Gi = "Gi" // 1*1024*1024*1024
	Ti = "Ti" // 1*1024*1024*1024*1024
	Pi = "Pi" // 1*1024*1024*1024*1024*1024
	Ei = "Ei" // 1*1024*1024*1024*1024*1024*1024
)

// BinaryUnitConvert convert data by given expect unit.
func BinaryUnitConvert(data, expectUnit string) (float64, error) {
	q, err := resource.ParseQuantity(data)
	if err != nil {
		return 0, err
	}
	v := float64(q.Value())

	var res float64

	switch expectUnit {
	case Ki:
		res = v / 1024
	case Mi:
		res = v / (1024 * 1024)
	case Gi:
		res = v / (1024 * 1024 * 1024)
	case Ti:
		res = v / (1024 * 1024 * 1024 * 1024)
	case Pi:
		res = v / (1024 * 1024 * 1024 * 1024 * 1024)
	case Ei:
		res = v / (1024 * 1024 * 1024 * 1024 * 1024 * 1024)
	default:
		return 0, fmt.Errorf("not support binary unit \"%v\"", expectUnit)
	}

	return res, nil
}
