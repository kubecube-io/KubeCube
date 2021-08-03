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

package transport

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"net/http"
)

// MakeTransport new transport with cert if given
func MakeTransport(rootCa, rootKey string) *http.Transport {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	if rootCa == "" || rootKey == "" {
		return tr
	}
	certs, err := tls.LoadX509KeyPair(rootCa, rootKey)
	if err != nil {
		clog.Warn("load tls cert failed, use Insecure: %v", err.Error())
	} else {
		ca, err := x509.ParseCertificate(certs.Certificate[0])
		if err != nil {
			return tr
		}
		pool := x509.NewCertPool()
		pool.AddCert(ca)

		tr = &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pool},
		}

	}
	return tr
}
