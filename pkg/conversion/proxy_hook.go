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

package conversion

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net"
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/streaming"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/endpoints/handlers/negotiation"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	restclientwatch "k8s.io/client-go/rest/watch"

	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/cubeproxy"
)

// VersionConvertDelegator delegate the hookDelegator of cube proxy to make hook.
type VersionConvertDelegator struct {
	converter *VersionConverter
}

func (d *VersionConvertDelegator) NewProxyHook(w http.ResponseWriter, r *http.Request) cubeproxy.ProxyHook {
	return &VersionConvertHook{
		converter: d.converter,
	}
}

// VersionConvertHook will be created as every request happen.
type VersionConvertHook struct {
	converter *VersionConverter
	ctx       hookContext
}

// hookContext contains the context during each request.
type hookContext struct {
	convertResponse     bool
	isWatch             bool
	rawNegotiator       runtime.ClientNegotiator
	convertedNegotiator runtime.ClientNegotiator
	rawGvr              *schema.GroupVersionResource
	convertedGvr        *schema.GroupVersionResource
}

func NewVersionConvertDelegator(cfg *rest.Config) (*VersionConvertDelegator, error) {
	cli, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, err
	}
	c, err := NewVersionConvertor(cli, nil)
	if err != nil {
		return nil, err
	}
	return &VersionConvertDelegator{converter: c}, nil
}

// NewForConfig make new rest.Config for user proxy.
func NewForConfig(cfg *rest.Config, opts cubeproxy.Options) (*cubeproxy.Config, error) {
	c, err := NewVersionConvertDelegator(cfg)
	if err != nil {
		return nil, err
	}
	opts.HookDelegator = c
	return cubeproxy.NewForConfig(cfg, opts), nil
}

func isWatchReq(req *http.Request) bool {
	qs := req.URL.Query()
	if qs.Get("watch") == "true" {
		return true
	}
	return false
}

func extractAcceptValue(h http.Header) string {
	v := h.Get("Accept")
	res := strings.Split(v, ",")
	return res[0]
}

func (c *VersionConvertHook) BeforeReqHook(req *http.Request) {
	needConvert, convertedObj, convertedUrl, err := c.tryVersionConvert(req)
	if err != nil {
		clog.Info("paas though cause %v", err)
		c.ctx.convertResponse = false
		return
	}

	// judge if watch request
	c.ctx.isWatch = isWatchReq(req)

	if needConvert {
		clog.Info("request verb: %v, watch enable: %v, convert request url: %v -> %v", req.Method, c.ctx.isWatch, req.URL.Path, convertedUrl)
		c.ctx.convertResponse = true
		// replace request body and url when need
		if convertedObj != nil {
			r := bytes.NewReader(convertedObj)
			body := io.NopCloser(r)
			req.Body = body
			req.ContentLength = int64(r.Len())
		}
		req.URL.Path = convertedUrl
	}

	// response need be converted except delete request
	if req.Method == http.MethodDelete {
		c.ctx.convertResponse = false
	}
}

func (c *VersionConvertHook) BeforeRespHook(resp *http.Response) error {
	if !c.ctx.convertResponse {
		return nil
	}

	contentType := resp.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return fmt.Errorf("unexpected content type from the server: %q: %v", contentType, err)
	}

	if c.ctx.isWatch {
		return c.convertStreamResp(resp, mediaType, params)
	}

	return c.convertShortResp(resp, mediaType, params)
}

func (c *VersionConvertHook) convertStreamResp(resp *http.Response, mediaType string, params map[string]string) error {
	// hijack the response stream using pipe
	rawBody := resp.Body
	r, w := net.Pipe()
	resp.Body = r

	objectDecoder, streamingSerializer, _, err := c.ctx.convertedNegotiator.StreamDecoder(mediaType, params)
	if err != nil {
		return err
	}

	serializer, err := negotiation.NegotiateInputSerializerForMediaType(mediaType, true, c.converter.cf)
	if err != nil {
		return err
	}

	// set up encoders and decoders
	framer := serializer.StreamSerializer.Framer
	streamSerializer := serializer.StreamSerializer.Serializer
	encoder := c.converter.cf.EncoderForVersion(streamSerializer, c.ctx.rawGvr.GroupVersion())
	objectEncoder := c.converter.cf.EncoderForVersion(serializer.Serializer, c.ctx.rawGvr.GroupVersion())
	framerWriter := framer.NewFrameWriter(w)
	framerReader := framer.NewFrameReader(rawBody)
	watchEventDecoder := streaming.NewDecoder(framerReader, streamingSerializer)
	streamEncoder := streaming.NewEncoder(framerWriter, encoder)

	// set up event watcher
	watcher := watch.NewStreamWatcher(
		restclientwatch.NewDecoder(watchEventDecoder, objectDecoder),
		errors.NewClientErrorReporter(http.StatusInternalServerError, resp.Request.Method, "ClientWatchDecoding"),
	)

	var unknown runtime.Unknown
	internalEvent := &metav1.InternalEvent{}
	outEvent := &metav1.WatchEvent{}
	buf := &bytes.Buffer{}

	go func() {
		defer func() {
			rawBody.Close() // close raw stream when read EOF.
			w.Close()       // close pipe writer to notify proxy handler to close resp body.
			watcher.Stop()  // close watcher when watch end.
			clog.Info("watch %s closed", c.ctx.rawGvr)
		}()
		for {
			select {
			case event, ok := <-watcher.ResultChan(): // watch event from raw stream
				if !ok {
					return
				}
				convertedObj, err := c.converter.Convert(event.Object, nil, c.ctx.rawGvr.GroupVersion())
				if err != nil {
					clog.Error("convert stream %s -> %s failed: %v", c.ctx.convertedGvr, c.ctx.rawGvr, err)
					return
				}
				if err = objectEncoder.Encode(convertedObj, buf); err != nil {
					clog.Error("encode object %v failed: %v", c.ctx.rawGvr, err)
					return
				}
				unknown.Raw = buf.Bytes()
				event.Object = &unknown
				*outEvent = metav1.WatchEvent{}
				*internalEvent = metav1.InternalEvent(event)
				err = metav1.Convert_v1_InternalEvent_To_v1_WatchEvent(internalEvent, outEvent, nil)
				if err != nil {
					clog.Error("convert internal event to watch event failed: %v", err)
					return
				}
				// write converted watchEvent back to stream
				if err = streamEncoder.Encode(outEvent); err != nil {
					clog.Error("encode watch stream for %v failed: %v", c.ctx.rawGvr, err)
					return
				}
				buf.Reset()
			}
		}
	}()

	return nil
}

func (c *VersionConvertHook) convertShortResp(resp *http.Response, mediaType string, params map[string]string) error {
	decoder, err := c.ctx.convertedNegotiator.Decoder(mediaType, params)
	if err != nil {
		return err
	}

	encoder, err := c.ctx.rawNegotiator.Encoder(mediaType, params)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	out, _, err := decoder.Decode(body, nil, nil)
	if err != nil {
		return err
	}

	switch out.(type) {
	case *metav1.Status:
		buf := bytes.NewBuffer(body)
		resp.Body = ioutil.NopCloser(buf)
		resp.Header["Content-Length"] = []string{fmt.Sprint(buf.Len())}
		return nil
	}

	convertedOut, err := c.converter.Convert(out, nil, c.ctx.rawGvr.GroupVersion())
	if err != nil {
		return err
	}

	newBody, err := runtime.Encode(encoder, convertedOut)
	if err != nil {
		return err
	}

	buf := bytes.NewBuffer(newBody)
	resp.Body = ioutil.NopCloser(buf)
	resp.Header["Content-Length"] = []string{fmt.Sprint(buf.Len())}
	return nil
}

func (c *VersionConvertHook) Error(w http.ResponseWriter, req *http.Request, err error) {
	clog.Error("Error while proxying request: %v", err)
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func (c *VersionConvertHook) tryVersionConvert(req *http.Request) (bool, []byte, string, error) {
	_, isNamespaced, gvr, err := ParseURL(req.URL.Path)
	if err != nil {
		clog.Debug("parse url % failed %v so pass through: %v", req.URL.Path, err)
		// pass through if we can not parse url
		return false, nil, "", nil
	}
	c.ctx.rawGvr = gvr
	greetBack, _, recommendVersion, err := c.converter.GvrGreeting(gvr)
	if err != nil {
		// we just record error and pass through anyway
		clog.Debug("object %v greeting cluster failed: %v, greet back is %v", gvr, err, greetBack)
	}
	if greetBack != IsNeedConvert {
		// pass through anyway if not need convert
		return false, nil, "", nil
	}
	if recommendVersion == nil {
		return false, nil, "", nil
	}

	// going here means that we need convert
	c.ctx.rawNegotiator = runtime.NewClientNegotiator(c.converter.cf, gvr.GroupVersion())
	c.ctx.convertedNegotiator = runtime.NewClientNegotiator(c.converter.cf, recommendVersion.GroupVersion())
	convertedGvr := recommendVersion.GroupVersion().WithResource(gvr.Resource)
	c.ctx.convertedGvr = &convertedGvr

	contentType := extractAcceptValue(req.Header)

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false, nil, "", fmt.Errorf("parse midia type %v failed: %v", contentType, err)
	}

	decoder, err := c.ctx.rawNegotiator.Decoder(mediaType, params)
	if err != nil {
		return false, nil, "", err
	}

	encoder, err := c.ctx.convertedNegotiator.Encoder(mediaType, params)
	if err != nil {
		return false, nil, "", err
	}

	// convert url according to specified rawGvr at first
	convertedUrl, err := ConvertURL(req.URL.Path, &convertedGvr)
	if err != nil {
		return false, nil, "", err
	}
	// we do not need convert body if request not create and update
	if req.Method != http.MethodPost && req.Method != http.MethodPut {
		return true, nil, convertedUrl, nil
	}

	// going here means that we need concert request body
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return false, nil, "", err
	}
	// decode data into internal version of object
	raw, rawGvr, err := decoder.Decode(data, nil, nil)
	if err != nil {
		return false, nil, "", err
	}
	if rawGvr.GroupVersion().String() != gvr.GroupVersion().String() {
		return false, nil, "", fmt.Errorf("gv parse failed with pair(%s~%s)", rawGvr.GroupVersion(), gvr.GroupVersion())
	}
	// covert internal version object int recommend version object
	out, err := c.converter.Convert(raw, nil, recommendVersion.GroupVersion())
	if err != nil {
		return false, nil, "", err
	}
	// encode concerted object
	codec := c.converter.cf.EncoderForVersion(encoder, recommendVersion.GroupVersion())
	convertedObj, err := runtime.Encode(codec, out)
	if err != nil {
		return false, nil, "", err
	}

	objMeta, err := meta.Accessor(out)
	if err != nil {
		return false, nil, "", err
	}

	if isNamespaced {
		clog.Debug("resource (%v/%v) converted with (%s~%s) when visit cluster", objMeta.GetNamespace(), objMeta.GetName(), gvr, convertedGvr)
	} else {
		clog.Debug("resource (%v) converted with (%v~%v) when visit cluster", objMeta.GetName(), gvr, convertedGvr)
	}

	return true, convertedObj, convertedUrl, nil
}
