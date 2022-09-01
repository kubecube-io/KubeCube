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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	v1 "k8s.io/api/apps/v1"
	"k8s.io/api/apps/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	restclient "k8s.io/client-go/rest"
)

func newFakeVersionConvert(t *testing.T) *VersionConverter {
	stable := metav1.APIResourceList{
		GroupVersion: "v1",
		APIResources: []metav1.APIResource{
			{Name: "pods", Namespaced: true, Kind: "Pod"},
		},
	}

	extensions := metav1.APIResourceList{
		GroupVersion: "extensions/v1beta1",
		APIResources: []metav1.APIResource{
			{Name: "deployments", Namespaced: true, Kind: "Deployment"},
		},
	}

	beta1 := metav1.APIResourceList{
		GroupVersion: "apps/v1beta1",
		APIResources: []metav1.APIResource{
			{Name: "deployments", Namespaced: true, Kind: "Deployment"},
		},
	}

	beta2 := metav1.APIResourceList{
		GroupVersion: "apps/v1beta2",
		APIResources: []metav1.APIResource{
			{Name: "deployments", Namespaced: true, Kind: "Deployment"},
		},
	}

	cronjobV1beta1 := metav1.APIResourceList{
		GroupVersion: "batch/v1beta1",
		APIResources: []metav1.APIResource{
			{Name: "cronjobs", Namespaced: true, Kind: "CronJob"},
		},
	}

	jobV1 := metav1.APIResourceList{
		GroupVersion: "batch/v1",
		APIResources: []metav1.APIResource{
			{Name: "Job", Namespaced: true, Kind: "Job"},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var list interface{}
		switch req.URL.Path {
		case "/apis":
			list = &metav1.APIGroupList{
				Groups: []metav1.APIGroup{
					{
						Name: "apps",
						Versions: []metav1.GroupVersionForDiscovery{
							{GroupVersion: "apps/v1beta1", Version: "v1beta1"},
							{GroupVersion: "apps/v1beta2", Version: "v1beta2"},
						},
						PreferredVersion: metav1.GroupVersionForDiscovery{
							GroupVersion: "apps/v1beta1",
							Version:      "v1beta1",
						},
					},
					{
						Name: "extensions",
						Versions: []metav1.GroupVersionForDiscovery{
							{GroupVersion: "extensions/v1beta1", Version: "v1beta1"},
						},
						PreferredVersion: metav1.GroupVersionForDiscovery{
							GroupVersion: "extensions/v1beta1",
							Version:      "v1beta1",
						},
					},
					{
						Name: "batch",
						Versions: []metav1.GroupVersionForDiscovery{
							{GroupVersion: "batch/v1beta1", Version: "v1beta1"},
							{GroupVersion: "batch/v1", Version: "v1"},
						},
						PreferredVersion: metav1.GroupVersionForDiscovery{
							GroupVersion: "batch/v1",
							Version:      "v1",
						},
					},
				},
			}
		case "/api":
			list = &metav1.APIVersions{
				Versions: []string{
					"v1",
				},
			}
		case "/api/v1":
			list = &stable
		case "/apis/extensions/v1beta1":
			list = &extensions
		case "/apis/apps/v1beta1":
			list = &beta1
		case "/apis/apps/v1beta2":
			list = &beta2
		case "/apis/batch/v1beta1":
			list = &cronjobV1beta1
		case "/apis/batch/v1":
			list = &jobV1
		case "/version":
			list = &version.Info{}
		default:
			t.Logf("unexpected request: %s", req.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		output, err := json.Marshal(list)
		if err != nil {
			t.Errorf("unexpected encoding error: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(output)
	}))

	d := discovery.NewDiscoveryClientForConfigOrDie(&restclient.Config{Host: server.URL})
	c, err := NewVersionConvertor(d, nil)
	if err != nil {
		t.Logf("new version convert failed: %v", err)
		return nil
	}
	return c
}

func TestConvertURL(t *testing.T) {
	const (
		testGroup   = "test-group"
		testVersion = "test-version"
	)

	tests := []struct {
		name             string
		url              string
		gvr              *schema.GroupVersionResource
		wantConvertedUrl string
		wantErr          bool
	}{
		{
			name:             "core namespaced api",
			url:              "/api/v1/namespaces/default/pods",
			gvr:              &schema.GroupVersionResource{Version: testVersion, Resource: "pods"},
			wantConvertedUrl: "/api/test-version/namespaces/default/pods",
			wantErr:          false,
		},
		{
			name:             "core cluster api",
			url:              "/api/v1/namespaces/test-ns",
			gvr:              &schema.GroupVersionResource{Version: testVersion, Resource: "namespaces"},
			wantConvertedUrl: "/api/test-version/namespaces/test-ns",
			wantErr:          false,
		},
		{
			name:             "no-core namespaced api",
			url:              "/apis/batch/v1/namespaces/default/jobs",
			gvr:              &schema.GroupVersionResource{Group: testGroup, Version: testVersion, Resource: "jobs"},
			wantConvertedUrl: "/apis/test-group/test-version/namespaces/default/jobs",
			wantErr:          false,
		},
		{
			name:             "no-core cluster api",
			url:              "/apis/rbac.authorization.k8s.io/v1/clusterroles",
			gvr:              &schema.GroupVersionResource{Group: testGroup, Version: testVersion, Resource: "clusterroles"},
			wantConvertedUrl: "/apis/test-group/test-version/clusterroles",
			wantErr:          false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotConvertedUrl, err := ConvertURL(tt.url, tt.gvr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotConvertedUrl != tt.wantConvertedUrl {
				t.Errorf("ConvertURL() gotConvertedUrl = %v, isCoreApi %v", gotConvertedUrl, tt.wantConvertedUrl)
			}
		})
	}
}

func TestGvk2Gvr(t *testing.T) {
	tests := []struct {
		name    string
		gvk     *schema.GroupVersionKind
		want    *schema.GroupVersionResource
		wantErr bool
	}{
		{
			name:    "normal gvk",
			gvk:     &schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			want:    &schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			wantErr: false,
		},
		{
			name:    "unknown group",
			gvk:     &schema.GroupVersionKind{Group: "unknown", Version: "v1", Kind: "Deployment"},
			wantErr: true,
		},
		{
			name:    "unknown version",
			gvk:     &schema.GroupVersionKind{Group: "apps", Version: "unknown", Kind: "Deployment"},
			wantErr: true,
		},
		{
			name:    "unknown kind",
			gvk:     &schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "unknown"},
			wantErr: true,
		},
		{
			name:    "unknown gvk",
			gvk:     &schema.GroupVersionKind{Group: "unknown", Version: "unknown", Kind: "unknown"},
			wantErr: true,
		},
	}
	c := newFakeVersionConvert(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Gvk2Gvr(c.RestMapper, tt.gvk)
			if (err != nil) != tt.wantErr {
				t.Errorf("Gvk2Gvr() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Gvk2Gvr() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGvr2Gvk(t *testing.T) {
	tests := []struct {
		name    string
		gvr     *schema.GroupVersionResource
		want    *schema.GroupVersionKind
		wantErr bool
	}{
		{
			name:    "normal gvr",
			gvr:     &schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			want:    &schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			wantErr: false,
		},
		{
			name:    "unknown group",
			gvr:     &schema.GroupVersionResource{Group: "unknown", Version: "v1", Resource: "deployments"},
			wantErr: true,
		},
		{
			name:    "unknown version",
			gvr:     &schema.GroupVersionResource{Group: "apps", Version: "unknown", Resource: "deployments"},
			wantErr: true,
		},
		{
			name:    "unknown resources",
			gvr:     &schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "unknown"},
			wantErr: true,
		},
		{
			name:    "unknown gvr",
			gvr:     &schema.GroupVersionResource{Group: "unknown", Version: "unknown", Resource: "unknown"},
			wantErr: true,
		},
	}
	c := newFakeVersionConvert(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Gvr2Gvk(c.RestMapper, tt.gvr)
			if (err != nil) != tt.wantErr {
				t.Errorf("Gvr2Gvk() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Gvr2Gvk() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsStableVersion(t *testing.T) {
	tests := []struct {
		name string
		gv   schema.GroupVersion
		want bool
	}{
		{
			name: "v1",
			gv:   schema.GroupVersion{Version: "v1"},
			want: true,
		},
		{
			name: "v999",
			gv:   schema.GroupVersion{Version: "v999"},
			want: true,
		},
		{
			name: "v1alpha1",
			gv:   schema.GroupVersion{Version: "v1alpha1"},
			want: false,
		},
		{
			name: "apps/v1",
			gv:   schema.GroupVersion{Group: "apps", Version: "v1"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsStableVersion(tt.gv); got != tt.want {
				t.Errorf("IsStableVersion() = %v, isCoreApi %v", got, tt.want)
			}
		})
	}
}

func TestParseURL(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		isCoreApi    bool
		isNamespaced bool
		want         *schema.GroupVersionResource
		wantErr      bool
	}{
		{
			name:         "core namespaced api",
			url:          "/api/v1/namespaces/default/pods",
			isCoreApi:    true,
			isNamespaced: true,
			want:         &schema.GroupVersionResource{Version: "v1", Resource: "pods"},
			wantErr:      false,
		},
		{
			name:         "core cluster api",
			url:          "/api/v1/namespaces/ns1",
			isCoreApi:    true,
			isNamespaced: false,
			want:         &schema.GroupVersionResource{Version: "v1", Resource: "namespaces"},
			wantErr:      false,
		},
		{
			name:         "no-core namespace api",
			url:          "/apis/batch/v1/namespaces/default/jobs",
			isCoreApi:    false,
			isNamespaced: true,
			want:         &schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"},
			wantErr:      false,
		},
		{
			name:         "no-core cluster api",
			url:          "/apis/rbac.authorization.k8s.io/v1/clusterroles",
			isCoreApi:    false,
			isNamespaced: false,
			want:         &schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
			wantErr:      false,
		},
		{
			name:    "incorrect prefix no /",
			url:     "api/v1/namespaces/default/pods",
			wantErr: true,
		},
		{
			name:    "no k8s prefix",
			url:     "apizzz/v1/namespaces/default/pods",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got2, err := ParseURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.isCoreApi {
				t.Errorf("ParseURL() got = %v, isCoreApi %v", got, tt.isCoreApi)
			}
			if got1 != tt.isNamespaced {
				t.Errorf("ParseURL() got1 = %v, isCoreApi %v", got1, tt.isNamespaced)
			}
			if !reflect.DeepEqual(got2, tt.want) {
				t.Errorf("ParseURL() got2 = %v, isCoreApi %v", got2, tt.want)
			}
		})
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name             string
		version          string
		wantCurrentMajor int
		wantCurrentMinor int
		wantErr          bool
	}{
		{
			name:             "v1.19",
			version:          "v1.19",
			wantCurrentMajor: 1,
			wantCurrentMinor: 19,
			wantErr:          false,
		},
		{
			name:             "1.19",
			version:          "1.19",
			wantCurrentMajor: 1,
			wantCurrentMinor: 19,
			wantErr:          false,
		},
		{
			name:    "x1.19",
			version: "x1.19",
			wantErr: true,
		},
		{
			name:    "1.19.1",
			version: "1.19.1",
			wantErr: true,
		},
		{
			name:    "v1.19.1",
			version: "v1.19.1",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCurrentMajor, gotCurrentMinor, err := ParseVersion(tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotCurrentMajor != tt.wantCurrentMajor {
				t.Errorf("ParseVersion() gotCurrentMajor = %v, isCoreApi %v", gotCurrentMajor, tt.wantCurrentMajor)
			}
			if gotCurrentMinor != tt.wantCurrentMinor {
				t.Errorf("ParseVersion() gotCurrentMinor = %v, isCoreApi %v", gotCurrentMinor, tt.wantCurrentMinor)
			}
		})
	}
}

func TestVersionCompare(t *testing.T) {
	tests := []struct {
		name    string
		v1      string
		v2      string
		want    int
		wantErr bool
	}{
		{
			name:    "v1 > v2",
			v1:      "1.20",
			v2:      "1.19",
			want:    1,
			wantErr: false,
		},
		{
			name:    "v1 = v2",
			v1:      "v1.19",
			v2:      "v1.19",
			want:    0,
			wantErr: false,
		},
		{
			name:    "v1 < v2",
			v1:      "v1.19",
			v2:      "v1.20",
			want:    -1,
			wantErr: false,
		},
		{
			name:    "error format1",
			v1:      "x1.19",
			v2:      "v1.19",
			wantErr: true,
		},
		{
			name:    "error format2",
			v1:      "v1.19.1",
			v2:      "1.19",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := VersionCompare(tt.v1, tt.v2)
			if (err != nil) != tt.wantErr {
				t.Errorf("VersionCompare() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("VersionCompare() got = %v, isCoreApi %v", got, tt.want)
			}
		})
	}
}

func TestVersionConverter_DirectConvert(t *testing.T) {
	tests := []struct {
		name    string
		in      runtime.Object
		out     runtime.Object
		target  runtime.GroupVersioner
		want    reflect.Type
		wantErr bool
	}{
		{
			name:    "old to new",
			in:      &v1beta1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "test"}},
			out:     &v1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "test"}},
			target:  &schema.GroupVersion{Group: "apps", Version: "v1"},
			want:    reflect.TypeOf(&v1.Deployment{}),
			wantErr: false,
		},
		{
			name:    "new to old",
			in:      &v1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "test"}},
			out:     &v1beta1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "test"}},
			target:  &schema.GroupVersion{Group: "apps", Version: "v1beta1"},
			want:    reflect.TypeOf(&v1beta1.Deployment{}),
			wantErr: false,
		},
		{
			name:    "convert to unknown",
			in:      &v1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "test"}},
			out:     nil,
			target:  &schema.GroupVersion{Group: "unknown", Version: "unknown"},
			wantErr: true,
		},
	}
	c := newFakeVersionConvert(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.DirectConvert(tt.in, tt.out, tt.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("DirectConvert() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if reflect.TypeOf(got) != tt.want {
				t.Errorf("DirectConvert() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVersionConverter_GvkGreeting(t *testing.T) {
	tests := []struct {
		name             string
		gvk              *schema.GroupVersionKind
		wantGreetBack    GreetBackType
		wantRawGvk       *schema.GroupVersionKind
		wantRecommendGvk *schema.GroupVersionKind
		wantErr          bool
	}{
		{
			name:             "pass through",
			gvk:              &schema.GroupVersionKind{Group: "apps", Version: "v1beta1", Kind: "Deployment"},
			wantGreetBack:    IsPassThrough,
			wantRawGvk:       &schema.GroupVersionKind{Group: "apps", Version: "v1beta1", Kind: "Deployment"},
			wantRecommendGvk: nil,
			wantErr:          false,
		},
		{
			name:             "had recommend version",
			gvk:              &schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			wantGreetBack:    IsNeedConvert,
			wantRawGvk:       &schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			wantRecommendGvk: &schema.GroupVersionKind{Group: "apps", Version: "v1beta1", Kind: "Deployment"},
			wantErr:          false,
		},
		{
			name:             "list had recommend version",
			gvk:              &schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DeploymentList"},
			wantGreetBack:    IsNeedConvert,
			wantRawGvk:       &schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DeploymentList"},
			wantRecommendGvk: &schema.GroupVersionKind{Group: "apps", Version: "v1beta1", Kind: "DeploymentList"},
			wantErr:          false,
		},
		{
			name:             "unavailable group",
			gvk:              &schema.GroupVersionKind{Group: "unknown", Version: "v1", Kind: "Deployment"},
			wantGreetBack:    IsNeedConvert,
			wantRawGvk:       &schema.GroupVersionKind{Group: "unknown", Version: "v1", Kind: "Deployment"},
			wantRecommendGvk: &schema.GroupVersionKind{Group: "apps", Version: "v1beta1", Kind: "Deployment"},
			wantErr:          false,
		},
		{
			name:             "unavailable version",
			gvk:              &schema.GroupVersionKind{Group: "apps", Version: "unknown", Kind: "Deployment"},
			wantGreetBack:    IsNeedConvert,
			wantRawGvk:       &schema.GroupVersionKind{Group: "apps", Version: "unknown", Kind: "Deployment"},
			wantRecommendGvk: &schema.GroupVersionKind{Group: "apps", Version: "v1beta1", Kind: "Deployment"},
			wantErr:          false,
		},
		{
			name:             "unavailable kind",
			gvk:              &schema.GroupVersionKind{Group: "unknown", Version: "unknown", Kind: "unknown"},
			wantGreetBack:    IsNotSupport,
			wantRawGvk:       &schema.GroupVersionKind{Group: "unknown", Version: "unknown", Kind: "unknown"},
			wantRecommendGvk: nil,
			wantErr:          false,
		},
		{
			name:             "not pass through",
			gvk:              &schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "CronJob"},
			wantGreetBack:    IsNeedConvert,
			wantRawGvk:       &schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "CronJob"},
			wantRecommendGvk: &schema.GroupVersionKind{Group: "batch", Version: "v1beta1", Kind: "CronJob"},
			wantErr:          false,
		},
	}
	c := newFakeVersionConvert(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			greetBack, gotRawGvk, gotRecommendGvk, err := c.GvkGreeting(tt.gvk)
			if (err != nil) != tt.wantErr {
				t.Errorf("GvkGreeting() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if greetBack != tt.wantGreetBack {
				t.Errorf("GvkGreeting() greetBack = %v, want %v", greetBack, tt.wantGreetBack)
			}
			if !reflect.DeepEqual(gotRawGvk, tt.wantRawGvk) {
				t.Errorf("GvkGreeting() gotRawGvk = %v, want %v", gotRawGvk, tt.wantRawGvk)
			}
			if !reflect.DeepEqual(gotRecommendGvk, tt.wantRecommendGvk) {
				t.Errorf("GvkGreeting() gotRecommendGvk = %v, want %v", gotRecommendGvk, tt.wantRecommendGvk)
			}
		})
	}
}
