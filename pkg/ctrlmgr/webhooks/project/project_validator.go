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

package project

import (
	"context"
	"net/http"

	v1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	tenantv1 "github.com/kubecube-io/kubecube/pkg/apis/tenant/v1"
	"github.com/kubecube-io/kubecube/pkg/clog"
)

type Validator struct {
	Client  client.Client
	decoder *admission.Decoder
}

func NewValidator(client client.Client, decoder *admission.Decoder) *Validator {
	return &Validator{
		Client:  client,
		decoder: decoder,
	}
}

func (r *Validator) Handle(ctx context.Context, req admission.Request) admission.Response {
	oldProject := tenantv1.Project{}
	currentProject := tenantv1.Project{}
	switch req.Operation {
	case v1.Create:
		err := r.decoder.Decode(req, &currentProject)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		err = r.ValidateCreate(&currentProject)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		return admission.Allowed("")
	case v1.Update:
		err := r.decoder.Decode(req, &currentProject)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		err = r.decoder.DecodeRaw(req.OldObject, &oldProject)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		err = r.ValidateUpdate(&oldProject, &currentProject)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		return admission.Allowed("")
	case v1.Delete:
		err := r.decoder.DecodeRaw(req.OldObject, &oldProject)
		if err != nil {
			clog.Error(err.Error())
			return admission.Errored(http.StatusBadRequest, err)
		}
		err = r.ValidateDelete(&oldProject)
		if err != nil {
			clog.Error(err.Error())
			return admission.Errored(http.StatusBadRequest, err)
		}
		return admission.Allowed("")
	}
	return admission.Allowed("")
}
