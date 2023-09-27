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
	"os"
	"net"
	"net/http"
	"time"
)

func DefaultTransport() *http.Transport {
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          50,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

func DefaultTransportOpts(tr *http.Transport) {
	tr.DialContext = (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext
	tr.ForceAttemptHTTP2 = true
	tr.MaxIdleConns = 50
	tr.IdleConnTimeout = 60 * time.Second
	tr.TLSHandshakeTimeout = 10 * time.Second
	tr.ExpectContinueTimeout = 1 * time.Second
}

func MakeInsecureTransport() *http.Transport {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	DefaultTransportOpts(tr)

	return tr
}

func MakeTlsTransportByFile(caFile string) (*http.Transport, error) {
	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}

	return MakeTlsTransport(caCert)
}

func MakeMTlsTransportByFile(caFile, certFile, keyFile string) (*http.Transport, error) {
	caCert, err := os.ReadFile(caFile)
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

	tr := &http.Transport{TLSClientConfig: &tls.Config{
		RootCAs: pool,
	}}

	DefaultTransportOpts(tr)

	return tr, nil
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

	tr := &http.Transport{TLSClientConfig: &tls.Config{
		RootCAs:      pool,
		Certificates: []tls.Certificate{clientCert},
	}}

	DefaultTransportOpts(tr)

	return tr, nil
}
