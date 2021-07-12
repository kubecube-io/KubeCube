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

package quota

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	quotav1 "github.com/kubecube-io/kubecube/pkg/apis/quota/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/quota/cube"
	v1 "k8s.io/api/admission/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// CubeResourceQuotaValidator guarantee cube resource quota not exceed
// the limit of parent cube resource quota
type CubeResourceQuotaValidator struct {
	Client  client.Client
	decoder *admission.Decoder
}

func (r *CubeResourceQuotaValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	oldQuota := &quotav1.CubeResourceQuota{}
	currentQuota := &quotav1.CubeResourceQuota{}

	defer func() {
		clog.Debug("operation: %v, current quota: %+v, old quota: %+v", req.Operation, currentQuota, oldQuota)
	}()

	switch req.Operation {
	case v1.Create:
		err := r.decoder.Decode(req, currentQuota)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		oldQuota = nil
	case v1.Update:
		err := r.decoder.Decode(req, currentQuota)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		err = r.decoder.DecodeRaw(req.OldObject, oldQuota)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if !cube.AllowedUpdate(currentQuota, oldQuota) {
			reason := fmt.Sprintf("hard of cube resource quota %v should not less than used", currentQuota.Name)
			clog.Warn(reason)
			return admission.Denied(reason)
		}
	case v1.Delete:
		err := r.decoder.DecodeRaw(req.OldObject, oldQuota)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if !cube.AllowedDel(oldQuota) {
			reason := fmt.Sprintf("must delete sub resource of cube resource quota %v first", oldQuota.Name)
			clog.Warn(reason)
			return admission.Errored(http.StatusNotAcceptable, errors.New(reason))
		}

		currentQuota = nil
	}

	q := cube.NewQuotaOperator(r.Client, currentQuota, oldQuota, context.Background())

	if req.Operation != v1.Delete {
		isOverLoad, err := q.Overload()
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		if isOverLoad {
			reason := fmt.Sprintf("request of cube resource quota overload")
			clog.Warn(reason)
			return admission.Errored(http.StatusNotAcceptable, errors.New(reason))
		}
	}

	// we consider the object finally create successful if UID was generated
	if cube.IsRelyOnObj(currentQuota, oldQuota) {
		go callback(q, req.Operation == v1.Delete)
	}

	return admission.Allowed("")
}

// InjectDecoder injects the decoder.
func (r *CubeResourceQuotaValidator) InjectDecoder(d *admission.Decoder) error {
	r.decoder = d
	return nil
}
