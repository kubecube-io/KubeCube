package conversion

import (
	"github.com/kubecube-io/kubecube/pkg/cubeproxy"
	"k8s.io/client-go/rest"
	"net/http"
)

type VersionConvertHook struct {
	converter VersionConverter
}

func NewVersionConvertHook() *VersionConvertHook {
	return nil
}

func NewForConfig(cfg *rest.Config, opts cubeproxy.Options) *cubeproxy.Config {
	c := NewVersionConvertHook()
	opts.BeforeReqHook = c.BeforeReqHook
	opts.BeforeRespHook = c.BeforeRespHook
	return cubeproxy.NewForConfig(cfg, opts)
}

func (c VersionConvertHook) BeforeReqHook(req *http.Request) error {
	return nil
}

func (c VersionConvertHook) BeforeRespHook(resp http.ResponseWriter) error {
	return nil
}
