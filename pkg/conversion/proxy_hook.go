package conversion

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net"
	"net/http"

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

type VersionConvertDelegator struct {
	converter *VersionConverter
}

func (d *VersionConvertDelegator) NewProxyHook(w http.ResponseWriter, r *http.Request) cubeproxy.ProxyHook {
	return &VersionConvertHook{
		converter: d.converter,
	}
}

type VersionConvertHook struct {
	converter *VersionConverter
	ctx       hookContext
}

type hookContext struct {
	convertResponse     bool
	isWatch             bool
	rawNegotiator       runtime.ClientNegotiator
	convertedNegotiator runtime.ClientNegotiator
	gvr                 *schema.GroupVersionResource
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

func NewForConfig(cfg *rest.Config, opts cubeproxy.Options) (*cubeproxy.Config, error) {
	c, err := NewVersionConvertDelegator(cfg)
	if err != nil {
		return nil, err
	}
	opts.HookDelegator = c
	return cubeproxy.NewForConfig(cfg, opts), nil
}

func IsWatchReq(req *http.Request) bool {
	return false
}

func (c *VersionConvertHook) BeforeReqHook(req *http.Request) {
	needConvert, convertedObj, convertedUrl, err := c.tryVersionConvert(req)
	if err != nil {
		clog.Info("paas though cause %v", err)
		c.ctx.convertResponse = false
		return
	}

	qs := req.URL.Query()
	if qs.Get("watch") == "true" {
		c.ctx.isWatch = true
	} else {
		c.ctx.isWatch = false
	}

	if needConvert {
		clog.Info("convert request url: %v", req.URL.Path)
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
		clog.Error(err.Error())
		return err
	}

	// set up encoders and decoders
	framer := serializer.StreamSerializer.Framer
	streamSerializer := serializer.StreamSerializer.Serializer
	encoder := c.converter.cf.EncoderForVersion(streamSerializer, c.ctx.gvr.GroupVersion())
	objectEncoder := c.converter.cf.EncoderForVersion(serializer.Serializer, c.ctx.gvr.GroupVersion())
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
		defer rawBody.Close() // close raw stream when read EOF.
		defer w.Close()       // close pipe writer to notify proxy handler to close resp body.
		defer watcher.Stop()  // close watcher when watch end.
		for {
			select {
			case event, ok := <-watcher.ResultChan(): // watch event from raw stream
				if !ok {
					return
				}
				convertedObj, err := c.converter.Convert(event.Object, nil, c.ctx.gvr.GroupVersion())
				if err != nil {
					clog.Error(err.Error())
					return
				}
				if err = objectEncoder.Encode(convertedObj, buf); err != nil {
					clog.Error(err.Error())
					return
				}
				unknown.Raw = buf.Bytes()
				event.Object = &unknown
				*outEvent = metav1.WatchEvent{}
				*internalEvent = metav1.InternalEvent(event)
				err = metav1.Convert_v1_InternalEvent_To_v1_WatchEvent(internalEvent, outEvent, nil)
				if err != nil {
					clog.Error(err.Error())
					return
				}
				// write converted watchEvent back to stream
				if err = streamEncoder.Encode(outEvent); err != nil {
					clog.Error(err.Error())
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

	convertedOut, err := c.converter.Convert(out, nil, c.ctx.gvr.GroupVersion())
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
		clog.Debug(err.Error())
		return false, nil, "", nil
	}
	c.ctx.gvr = gvr
	greetBack, _, recommendVersion, err := c.converter.GvrGreeting(gvr)
	if err != nil {
		// we just record error and pass through anyway
		clog.Warn(err.Error())
	}
	if greetBack != IsNeedConvert {
		// pass through anyway if not need convert
		clog.Debug("%v greet cluster is %v, pass through", gvr.String(), greetBack)
		return false, nil, "", nil
	}
	if recommendVersion == nil {
		return false, nil, "", nil
	}

	// going here means that we need convert
	c.ctx.rawNegotiator = runtime.NewClientNegotiator(c.converter.cf, gvr.GroupVersion())
	c.ctx.convertedNegotiator = runtime.NewClientNegotiator(c.converter.cf, recommendVersion.GroupVersion())

	contentType := req.Header.Get("Content-Type")
	if len(contentType) == 0 {
		contentType = "application/vnd.kubernetes.protobuf"
		//contentType = "application/json"
	}
	//clog.Info("req header: %v", req.Header)
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

	// convert url according to specified gvr at first
	convertedUrl, err := ConvertURL(req.URL.Path, &schema.GroupVersionResource{Group: recommendVersion.Group, Version: recommendVersion.Version, Resource: gvr.Resource})
	if err != nil {
		return false, nil, "", err
	}
	// we do not need convert body if request not create and update
	if req.Method != http.MethodPost && req.Method != http.MethodPut {
		return true, nil, convertedUrl, nil
	}
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
		return false, nil, "", fmt.Errorf("gv parse failed with pair(%v~%v)", rawGvr.GroupVersion().String(), gvr.GroupVersion().String())
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
		clog.Info("resource (%v/%v) converted with (%v~%v) when visit cluster", objMeta.GetNamespace(), objMeta.GetName(), gvr.String(), recommendVersion.GroupVersion().WithResource(gvr.Resource))
	} else {
		clog.Info("resource (%v) converted with (%v~%v) when visit cluster", objMeta.GetName(), gvr.String(), recommendVersion.GroupVersion().WithResource(gvr.Resource))
	}

	return true, convertedObj, convertedUrl, nil
}
