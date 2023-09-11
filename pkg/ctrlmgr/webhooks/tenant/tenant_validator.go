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

package tenant

import (
	"context"
	"net/http"

	v1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	tenantv1 "github.com/kubecube-io/kubecube/pkg/apis/tenant/v1"
)

type Validator struct {
	decoder *admission.Decoder
}

func NewValidator(decoder *admission.Decoder) *Validator {
	return &Validator{
		decoder: decoder,
	}
}
func (r *Validator) Handle(ctx context.Context, req admission.Request) admission.Response {
	tenant := tenantv1.Tenant{}
	switch req.Operation {
	case v1.Create:
		return admission.Allowed("")
	case v1.Update:
		return admission.Allowed("")
	case v1.Delete:
		err := r.decoder.DecodeRaw(req.OldObject, &tenant)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		err = ValidateDelete(&tenant)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		return admission.Allowed("")
	}
	return admission.Allowed("")
}
