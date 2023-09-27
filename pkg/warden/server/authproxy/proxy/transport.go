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

// this file is lifted directly from k8s.io/apimachinery/util/proxy

package proxy

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"k8s.io/klog/v2"

	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/sets"
)

// atomsToAttrs states which attributes of which tags require URL substitution.
// Sources:
// http://www.w3.org/TR/REC-html40/index/attributes.html
// http://www.w3.org/html/wg/drafts/html/master/index.html#attributes-1
var atomsToAttrs = map[atom.Atom]sets.Set[string]{
	atom.A:          sets.New("href"),
	atom.Applet:     sets.New("codebase"),
	atom.Area:       sets.New("href"),
	atom.Audio:      sets.New("src"),
	atom.Base:       sets.New("href"),
	atom.Blockquote: sets.New("cite"),
	atom.Body:       sets.New("background"),
	atom.Button:     sets.New("formaction"),
	atom.Command:    sets.New("icon"),
	atom.Del:        sets.New("cite"),
	atom.Embed:      sets.New("src"),
	atom.Form:       sets.New("action"),
	atom.Frame:      sets.New("longdesc", "src"),
	atom.Head:       sets.New("profile"),
	atom.Html:       sets.New("manifest"),
	atom.Iframe:     sets.New("longdesc", "src"),
	atom.Img:        sets.New("longdesc", "src", "usemap"),
	atom.Input:      sets.New("src", "usemap", "formaction"),
	atom.Ins:        sets.New("cite"),
	atom.Link:       sets.New("href"),
	atom.Object:     sets.New("classid", "codebase", "data", "usemap"),
	atom.Q:          sets.New("cite"),
	atom.Script:     sets.New("src"),
	atom.Source:     sets.New("src"),
	atom.Video:      sets.New("poster", "src"),

	// TODO: css URLs hidden in style elements.
}

// Transport is a transport for text/html content that replaces URLs in html
// content with the prefix of the proxy server
type Transport struct {
	Scheme      string
	Host        string
	PathPrepend string

	http.RoundTripper
}

// RoundTrip implements the http.RoundTripper interface
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Add reverse proxy headers.
	forwardedURI := path.Join(t.PathPrepend, req.URL.Path)
	if strings.HasSuffix(req.URL.Path, "/") {
		forwardedURI = forwardedURI + "/"
	}
	req.Header.Set("X-Forwarded-Uri", forwardedURI)
	if len(t.Host) > 0 {
		req.Header.Set("X-Forwarded-Host", t.Host)
	}
	if len(t.Scheme) > 0 {
		req.Header.Set("X-Forwarded-Proto", t.Scheme)
	}

	rt := t.RoundTripper
	if rt == nil {
		rt = http.DefaultTransport
	}
	resp, err := rt.RoundTrip(req)

	if err != nil {
		return nil, fmt.Errorf("error trying to reach service: %w", err)
	}

	if redirect := resp.Header.Get("Location"); redirect != "" {
		resp.Header.Set("Location", t.rewriteURL(redirect, req.URL, req.Host))
		return resp, nil
	}

	cType := resp.Header.Get("Content-Type")
	cType = strings.TrimSpace(strings.SplitN(cType, ";", 2)[0])
	if cType != "text/html" {
		// Do nothing, simply pass through
		return resp, nil
	}

	return t.rewriteResponse(req, resp)
}

var _ = net.RoundTripperWrapper(&Transport{})

func (rt *Transport) WrappedRoundTripper() http.RoundTripper {
	return rt.RoundTripper
}

// rewriteURL rewrites a single URL to go through the proxy, if the URL refers
// to the same host as sourceURL, which is the page on which the target URL
// occurred, or if the URL matches the sourceRequestHost. If any error occurs (e.g.
// parsing), it returns targetURL.
func (t *Transport) rewriteURL(targetURL string, sourceURL *url.URL, sourceRequestHost string) string {
	url, err := url.Parse(targetURL)
	if err != nil {
		return targetURL
	}

	// Example:
	//      When API server processes a proxy request to a service (e.g. /api/v1/namespace/foo/service/bar/proxy/),
	//      the sourceURL.Host (i.e. req.URL.Host) is the endpoint IP address of the service. The
	//      sourceRequestHost (i.e. req.Host) is the Host header that specifies the host on which the
	//      URL is sought, which can be different from sourceURL.Host. For example, if user sends the
	//      request through "kubectl proxy" locally (i.e. localhost:8001/api/v1/namespace/foo/service/bar/proxy/),
	//      sourceRequestHost is "localhost:8001".
	//
	//      If the service's response URL contains non-empty host, and url.Host is equal to either sourceURL.Host
	//      or sourceRequestHost, we should not consider the returned URL to be a completely different host.
	//      It's the API server's responsibility to rewrite a same-host-and-absolute-path URL and append the
	//      necessary URL prefix (i.e. /api/v1/namespace/foo/service/bar/proxy/).
	isDifferentHost := url.Host != "" && url.Host != sourceURL.Host && url.Host != sourceRequestHost
	isRelative := !strings.HasPrefix(url.Path, "/")
	if isDifferentHost || isRelative {
		return targetURL
	}

	// Do not rewrite scheme and host if the Transport has empty scheme and host
	// when targetURL already contains the sourceRequestHost
	if !(url.Host == sourceRequestHost && t.Scheme == "" && t.Host == "") {
		url.Scheme = t.Scheme
		url.Host = t.Host
	}

	origPath := url.Path
	// Do not rewrite URL if the sourceURL already contains the necessary prefix.
	if strings.HasPrefix(url.Path, t.PathPrepend) {
		return url.String()
	}
	url.Path = path.Join(t.PathPrepend, url.Path)
	if strings.HasSuffix(origPath, "/") {
		// Add back the trailing slash, which was stripped by path.Join().
		url.Path += "/"
	}

	return url.String()
}

// rewriteHTML scans the HTML for tags with url-valued attributes, and updates
// those values with the urlRewriter function. The updated HTML is output to the
// writer.
func rewriteHTML(reader io.Reader, writer io.Writer, urlRewriter func(string) string) error {
	// Note: This assumes the content is UTF-8.
	tokenizer := html.NewTokenizer(reader)

	var err error
	for err == nil {
		tokenType := tokenizer.Next()
		switch tokenType {
		case html.ErrorToken:
			err = tokenizer.Err()
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			if urlAttrs, ok := atomsToAttrs[token.DataAtom]; ok {
				for i, attr := range token.Attr {
					if urlAttrs.Has(attr.Key) {
						token.Attr[i].Val = urlRewriter(attr.Val)
					}
				}
			}
			_, err = writer.Write([]byte(token.String()))
		default:
			_, err = writer.Write(tokenizer.Raw())
		}
	}
	if !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// rewriteResponse modifies an HTML response by updating absolute links referring
// to the original host to instead refer to the proxy transport.
func (t *Transport) rewriteResponse(req *http.Request, resp *http.Response) (*http.Response, error) {
	origBody := resp.Body
	defer origBody.Close()

	newContent := &bytes.Buffer{}
	var reader io.Reader = origBody
	var writer io.Writer = newContent
	encoding := resp.Header.Get("Content-Encoding")
	switch encoding {
	case "gzip":
		var err error
		reader, err = gzip.NewReader(reader)
		if err != nil {
			return nil, fmt.Errorf("errorf making gzip reader: %v", err)
		}
		gzw := gzip.NewWriter(writer)
		defer gzw.Close()
		writer = gzw
	case "deflate":
		var err error
		reader = flate.NewReader(reader)
		flw, err := flate.NewWriter(writer, flate.BestCompression)
		if err != nil {
			return nil, fmt.Errorf("errorf making flate writer: %v", err)
		}
		defer func() {
			flw.Close()
			flw.Flush()
		}()
		writer = flw
	case "":
		// This is fine
	default:
		// Some encoding we don't understand-- don't try to parse this
		klog.Errorf("Proxy encountered encoding %v for text/html; can't understand this so not fixing links.", encoding)
		return resp, nil
	}

	urlRewriter := func(targetUrl string) string {
		return t.rewriteURL(targetUrl, req.URL, req.Host)
	}
	err := rewriteHTML(reader, writer, urlRewriter)
	if err != nil {
		klog.Errorf("Failed to rewrite URLs: %v", err)
		return resp, err
	}

	resp.Body = io.NopCloser(newContent)
	// Update header node with new content-length
	// TODO: Remove any hash/signature headers here?
	resp.Header.Del("Content-Length")
	resp.ContentLength = int64(newContent.Len())

	return resp, err
}
