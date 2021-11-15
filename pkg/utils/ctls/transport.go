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

package ctls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

func MakeTlsTransportByFile(caFile string) (*http.Transport, error) {
	caCert, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, err
	}

	return MakeTlsTransport(caCert)
}

func MakeMTlsTransportByFile(caFile, certFile, keyFile string) (*http.Transport, error) {
	caCert, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, err
	}

	clientCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	return MakeMTlsTransport(caCert, clientCert)
}

func MakeTlsTransport(caCert []byte) (*http.Transport, error) {
	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(caCert); !ok {
		return nil, fmt.Errorf("parse cert failed: %v", caCert)
	}

	return &http.Transport{TLSClientConfig: &tls.Config{
		RootCAs: pool,
	}}, nil
}

func MakeMTlsTransportByPem(caCert, certData, keyData []byte) (*http.Transport, error) {
	clientCert, err := tls.X509KeyPair(certData, keyData)
	if err != nil {
		return nil, err
	}

	return MakeMTlsTransport(caCert, clientCert)
}

func MakeMTlsTransport(caCert []byte, clientCert tls.Certificate) (*http.Transport, error) {
	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(caCert); !ok {
		return nil, fmt.Errorf("parse cert failed: %v", caCert)
	}

	return &http.Transport{TLSClientConfig: &tls.Config{
		RootCAs:      pool,
		Certificates: []tls.Certificate{clientCert},
	},
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          50,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}, nil
}
