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

package domain

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation"
	
	"github.com/kubecube-io/kubecube/pkg/clog"
)

func ValidatorDomainSuffix(domainSuffixList []string, log clog.CubeLogger) error {
	if domainSuffixList == nil || len(domainSuffixList) == 0 {
		return nil
	}
	for _, domainSuffix := range domainSuffixList {
		if len(domainSuffix) != 0 {
			if errs := validation.IsDNS1123Subdomain(domainSuffix); len(errs) > 0 {
				log.Debug("Invalid value: %s ", domainSuffix)
				return fmt.Errorf("Invalid value: %s : a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')\n)", domainSuffix)
			}
		}
	}
	return nil
}
