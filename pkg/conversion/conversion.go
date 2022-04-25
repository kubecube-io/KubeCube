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
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/kubecube-io/kubecube/pkg/clog"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
)

// VersionConverter knows how to convert object to specified version
type VersionConverter struct {
	// scheme hold full versions k8s api
	scheme *runtime.Scheme

	// cf the codec factory had methods of encode and decode
	cf serializer.CodecFactory

	// discovery is response to communicate with k8s
	// todo: use cache client
	discovery discovery.DiscoveryInterface

	// clusterInfo hold the version info of target cluster
	clusterInfo *version.Info

	// RestMapper is response to map gvk and gvr
	RestMapper meta.RESTMapper
}

// NewVersionConvertor create a version convert for a target cluster
func NewVersionConvertor(discovery discovery.DiscoveryInterface, restMapper meta.RESTMapper, installFuncs ...InstallFunc) (*VersionConverter, error) {
	scheme := runtime.NewScheme()
	install(scheme, installFuncs...)

	info, err := discovery.ServerVersion()
	if err != nil {
		return nil, err
	}

	if restMapper == nil {
		m := meta.NewDefaultRESTMapper(scheme.PrioritizedVersionsAllGroups())
		for gvk := range scheme.AllKnownTypes() {
			m.Add(gvk, nil)
		}
		restMapper = m
	}

	return &VersionConverter{
		scheme:      scheme,
		discovery:   discovery,
		RestMapper:  restMapper,
		clusterInfo: info,
		cf:          serializer.NewCodecFactory(scheme),
	}, nil
}

// Convert converts an Object to another, generally the conversion is internalVersion <-> versioned.
// if out was set, the converted result would be set into.
func (c *VersionConverter) Convert(in runtime.Object, out runtime.Object, target runtime.GroupVersioner) (runtime.Object, error) {
	if out != nil {
		if err := c.scheme.Convert(in, out, target); err != nil {
			return nil, err
		}
		return out, nil
	}
	return c.scheme.ConvertToVersion(in, target)
}

// DirectConvert converts a versioned Object to another version with given target gv.
// if out was set, the converted result would be set into.
func (c *VersionConverter) DirectConvert(in runtime.Object, out runtime.Object, target runtime.GroupVersioner) (runtime.Object, error) {
	internalObject, err := c.Convert(in, nil, runtime.InternalGroupVersioner)
	if err != nil {
		return nil, err
	}
	if out != nil {
		if err := c.scheme.Convert(internalObject, out, target); err != nil {
			return nil, err
		}
		return out, nil
	}
	return c.Convert(internalObject, nil, target)
}

// GreetBackType will tell what the way that obj can access cluster
type GreetBackType int

const (
	IsPassThrough GreetBackType = iota
	IsNotSupport
	IsNeedConvert
	IsUnknown
)

// ObjectGreeting describes if given object is available in target cluster.
// a recommend group version kind will return if it cloud not pass through.
func (c *VersionConverter) ObjectGreeting(obj runtime.Object) (greetBack GreetBackType, rawGvk *schema.GroupVersionKind, recommendGvk *schema.GroupVersionKind, err error) {
	gvk := obj.GetObjectKind().GroupVersionKind()
	if gvk.Empty() {
		gvks, _, err := c.scheme.ObjectKinds(obj)
		if err != nil {
			return IsUnknown, nil, nil, err
		}
		gvk = gvks[0]
	}
	return c.GvkGreeting(&gvk)
}

// GvrGreeting describes if given gvr is available in target cluster.
// a recommend group version kind will return if it cloud not pass through.
func (c *VersionConverter) GvrGreeting(gvr *schema.GroupVersionResource) (greetBack GreetBackType, rawGvk *schema.GroupVersionKind, recommendGvk *schema.GroupVersionKind, err error) {
	gvk, err := Gvr2Gvk(c.RestMapper, gvr)
	if err != nil {
		return IsUnknown, gvk, nil, err
	}

	// use best priority
	return c.GvkGreeting(gvk)
}

// GvkGreeting describes if given gvk is available in target cluster.
// a recommend group version kind will return if it cloud not pass through.
func (c *VersionConverter) GvkGreeting(gvk *schema.GroupVersionKind) (greetBack GreetBackType, rawGvk *schema.GroupVersionKind, recommendGvk *schema.GroupVersionKind, err error) {
	clusterVersion := Version(c.clusterInfo)

	_, allResources, err := c.discovery.ServerGroupsAndResources()
	if err != nil {
		return IsUnknown, gvk, nil, err
	}
	if allResources != nil {
		for _, gvs := range allResources {
			for _, resource := range gvs.APIResources {
				if resource.Group == "" || resource.Version == "" {
					gv, err := schema.ParseGroupVersion(gvs.GroupVersion)
					if err != nil {
						return IsUnknown, gvk, nil, fmt.Errorf("parse group version %v failed: %v", gvs.GroupVersion, err)
					}
					resource.Group, resource.Version = gv.Group, gv.Version
				}
				if resource.Group == gvk.Group && resource.Version == gvk.Version && resource.Kind == gvk.Kind {
					// found match group/version/kind in target cluster.
					// so the object is available in target cluster.
					return IsPassThrough, gvk, nil, nil
				}
			}
		}
		for _, gvs := range allResources {
			for _, resource := range gvs.APIResources {
				if resource.Kind == gvk.Kind {
					preferredGroup := resource.Group
					preferredVersion := resource.Version
					if preferredGroup == "" || preferredVersion == "" {
						gv, err := schema.ParseGroupVersion(gvs.GroupVersion)
						if err != nil {
							return IsUnknown, gvk, nil, fmt.Errorf("parse group version %v failed: %v", gvs.GroupVersion, err)
						}
						preferredGroup, preferredVersion = gv.Group, gv.Version
					}
					// found object kind in target cluster.
					// Attention: if we had crd which kind is same with k8s kind
					// might cause problem, example: foo/bar.pod <--> apps/v1.pod
					return IsNeedConvert, gvk, &schema.GroupVersionKind{Group: preferredGroup, Version: preferredVersion, Kind: gvk.Kind}, nil
				}
			}
		}
	}

	clog.Debug("%v is not support in target cluster %v", gvk.String(), clusterVersion)

	return IsNotSupport, gvk, nil, nil
}

// Encode encodes given obj, generally the gv should match Object
func (c *VersionConverter) Encode(obj runtime.Object, gv runtime.GroupVersioner) ([]byte, error) {
	info, ok := runtime.SerializerInfoForMediaType(c.cf.SupportedMediaTypes(), runtime.ContentTypeJSON)
	if !ok {
		return nil, errors.New("no media type match for serializer")
	}
	encoder := info.Serializer
	codec := c.cf.EncoderForVersion(encoder, gv)
	out, err := runtime.Encode(codec, obj)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Decode decodes data to object, if defaults was not set, the internalVersion would be used.
func (c *VersionConverter) Decode(data []byte, defaults *schema.GroupVersionKind, into runtime.Object, versions ...schema.GroupVersion) (runtime.Object, *schema.GroupVersionKind, error) {
	decoder := c.cf.UniversalDecoder(versions...)
	return decoder.Decode(data, defaults, into)
}

// Gvr2Gvk convert gvr to gvk by specified cluster
func Gvr2Gvk(mapper meta.RESTMapper, gvr *schema.GroupVersionResource) (*schema.GroupVersionKind, error) {
	kinds, err := mapper.KindsFor(*gvr)
	if err != nil {
		return nil, err
	}

	if len(kinds) == 0 {
		return nil, fmt.Errorf("%v is not supportted", gvr.String())
	}

	// use best priority
	return &kinds[0], nil
}

// Gvk2Gvr convert gvk to gvr by specified cluster
func Gvk2Gvr(mapper meta.RESTMapper, gvk *schema.GroupVersionKind) (*schema.GroupVersionResource, error) {
	m, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}

	return &m.Resource, nil
}

// ConvertURL convert url by given gvr
func ConvertURL(url string, gvr *schema.GroupVersionResource) (convertedUrl string, err error) {
	const sep = "/"

	rawIsCoreApi, _, _, err := ParseURL(url)
	if err != nil {
		return "", err
	}

	isCoreApi := gvr.Group == ""

	ss := strings.Split(strings.TrimPrefix(url, sep), sep)

	switch {
	case isCoreApi && rawIsCoreApi:
		ss[1] = gvr.Version
	case !isCoreApi && rawIsCoreApi:
		ss[0] = "apis"
		ss[1] = gvr.Group + sep + gvr.Version
	case isCoreApi && !rawIsCoreApi:
		// /apis/batch/v1/namespaces/{namespace}/jobs
		ss[0] = "api"
		ss[2] = gvr.Version
		ss = append(ss[:1], ss[2:]...)
	case !isCoreApi && !rawIsCoreApi:
		ss[1] = gvr.Group
		ss[2] = gvr.Version
	}

	return sep + strings.Join(ss, "/"), nil
}

// ParseURL parse k8s api url into gvr
func ParseURL(url string) (bool, bool, *schema.GroupVersionResource, error) {
	invalidUrlErr := fmt.Errorf("url not k8s format: %s", url)

	const (
		coreApiPrefix    = "/api/"
		nonCoreApiPrefix = "/apis/"
		nsSubString      = "/namespaces/"
	)

	isCoreApi, isNonCoreApi := strings.HasPrefix(url, coreApiPrefix), strings.HasPrefix(url, nonCoreApiPrefix)

	ss := strings.Split(strings.TrimPrefix(url, "/"), "/")
	var isNamespaced bool
	if len(ss) > 4 && strings.Contains(url, nsSubString) {
		isNamespaced = true
	}

	gvr := &schema.GroupVersionResource{}
	switch {
	case isCoreApi && isNamespaced:
		// like: /api/v1/namespaces/{namespace}/pods
		if len(ss) < 5 {
			return false, false, nil, invalidUrlErr
		}
		gvr.Version = ss[1]
		gvr.Resource = ss[4]
	case isCoreApi && !isNamespaced:
		// like: /api/v1/namespaces/{name}
		if len(ss) < 3 {
			return false, false, nil, invalidUrlErr
		}
		gvr.Version = ss[1]
		gvr.Resource = ss[2]
	case isNonCoreApi && isNamespaced:
		// like: /apis/batch/v1/namespaces/{namespace}/jobs
		if len(ss) < 6 {
			return false, false, nil, invalidUrlErr
		}
		gvr.Group = ss[1]
		gvr.Version = ss[2]
		gvr.Resource = ss[5]
	case isNonCoreApi && !isNamespaced:
		// like: /apis/rbac.authorization.k8s.io/v1/clusterroles
		if len(ss) < 4 {
			return false, false, nil, invalidUrlErr
		}
		gvr.Group = ss[1]
		gvr.Version = ss[2]
		gvr.Resource = ss[3]
	default:
		return false, false, nil, invalidUrlErr
	}

	return isCoreApi, isNamespaced, gvr, nil
}

var stableVersionRegexp = regexp.MustCompile(`^[v]+[0-9]*$`)

// IsStableVersion tells if given gv is stable
func IsStableVersion(gv schema.GroupVersion) bool {
	if stableVersionRegexp.MatchString(gv.Version) && gv.Group == "" {
		return true
	}
	return false
}

// Version print cluster version info
func Version(info *version.Info) string {
	return fmt.Sprintf("%v.%v", info.Major, info.Minor)
}

// ParseVersion parse version to currentMajor and currentMinor
//
// only two format is valid, example:
// 1. v1.19
// 2. 1.19
func ParseVersion(version string) (currentMajor, currentMinor int, err error) {
	invalidError := errors.New("invalid version")

	if len(version) == 0 {
		return 0, 0, invalidError
	}

	v := strings.TrimLeft(version, "v")
	vs := strings.Split(v, ".")

	if len(vs) != 2 {
		return 0, 0, invalidError
	}

	currentMajor, err = strconv.Atoi(vs[0])
	if err != nil {
		return 0, 0, invalidError
	}

	currentMinor, err = strconv.Atoi(vs[1])
	if err != nil {
		return 0, 0, invalidError
	}

	return
}

// VersionCompare compare the both versions
// return 1 means v1 > v2
// return 0 means v1 = v2
// return -1 means v1 < v2
func VersionCompare(v1, v2 string) (int, error) {
	majorV1, minorV1, err := ParseVersion(v1)
	if err != nil {
		return 0, err
	}

	majorV2, minorV2, err := ParseVersion(v2)
	if err != nil {
		return 0, err
	}

	switch {
	case (majorV1 > majorV2) || (majorV1 == majorV2 && minorV1 > minorV2):
		return 1, nil
	case (majorV1 == majorV2) && (minorV1 == minorV2):
		return 0, nil
	default:
		return -1, nil
	}
}

// MarshalJSON marshal Unstructured into bytes
func MarshalJSON(u *unstructured.Unstructured) ([]byte, error) {
	return u.MarshalJSON()
}

// UnmarshalJSON unmarshal bytes into Unstructured
func UnmarshalJSON(b []byte) (*unstructured.Unstructured, error) {
	u := &unstructured.Unstructured{}
	err := u.UnmarshalJSON(b)
	if err != nil {
		return nil, err
	}
	return u, nil
}
