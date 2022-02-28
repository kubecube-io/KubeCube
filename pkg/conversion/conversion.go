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
	"github.com/kubecube-io/kubecube/pkg/multicluster"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/version"
	"strconv"
	"strings"
)

type VersionConverter struct {
	// Scheme hold full versions k8s api
	Scheme *runtime.Scheme

	// CF the codec factory had methods of encode and decode
	CF serializer.CodecFactory

	// MultiClusterMgr has methods that can communicate
	// with cluster it take over
	MultiClusterMgr multicluster.Manager
}

func NewVersionConvertor(mgr multicluster.Manager, installFuncs ...InstallFunc) *VersionConverter {
	scheme := runtime.NewScheme()
	install(scheme, installFuncs...)

	return &VersionConverter{
		Scheme:          scheme,
		MultiClusterMgr: mgr,
		CF:              serializer.NewCodecFactory(scheme),
	}
}

// Convert do real conversion action
func (c *VersionConverter) Convert(in runtime.Object, target runtime.GroupVersioner) (runtime.Object, error) {
	return c.Scheme.ConvertToVersion(in, target)
}

// Gvr2Gvk convert gvr to gvk by specified cluster
func (c *VersionConverter) Gvr2Gvk(gvr *schema.GroupVersionResource, targetCluster string) (*schema.GroupVersionKind, error) {
	cluster, err := c.MultiClusterMgr.Get(targetCluster)
	if err != nil {
		return nil, err
	}

	kinds, err := cluster.Client.RESTMapper().KindsFor(*gvr)
	if err != nil {
		return nil, err
	}

	if len(kinds) == 0 {
		return nil, fmt.Errorf("%v is not supportted in cluster %v", gvr.String(), targetCluster)
	}

	// use best priority
	return &kinds[0], nil
}

// Gvk2Gvr convert gvk to gvr by specified cluster
func (c *VersionConverter) Gvk2Gvr(gvk *schema.GroupVersionKind, targetCluster string) (*schema.GroupVersionResource, error) {
	cluster, err := c.MultiClusterMgr.Get(targetCluster)
	if err != nil {
		return nil, err
	}

	m, err := cluster.Client.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}

	return &m.Resource, nil
}

// IsObjectAvailable describes if given object is available in target cluster.
// a replacement group version kind will return if the result is unavailable.
func (c *VersionConverter) IsObjectAvailable(obj runtime.Object, targetCluster string) (bool, *schema.GroupVersionKind, error) {
	gvk := obj.GetObjectKind().GroupVersionKind()
	return c.IsGvkAvailable(&gvk, targetCluster)
}

// IsGvrAvailable describes if given gvr is available in target cluster.
// a replacement group version kind will return if the result is unavailable.
func (c *VersionConverter) IsGvrAvailable(gvr *schema.GroupVersionResource, targetCluster string) (bool, *schema.GroupVersionKind, error) {
	gvk, err := c.Gvr2Gvk(gvr, targetCluster)
	if err != nil {
		return false, nil, err
	}

	// use best priority
	return c.IsGvkAvailable(gvk, targetCluster)
}

// IsGvkAvailable describes if given gvk is available in target cluster.
// a replacement group version kind will return if the result is unavailable.
func (c *VersionConverter) IsGvkAvailable(gvk *schema.GroupVersionKind, targetCluster string) (bool, *schema.GroupVersionKind, error) {
	cluster, err := c.MultiClusterMgr.Get(targetCluster)
	if err != nil {
		return false, nil, err
	}

	clusterVersion := Version(cluster.Version)

	// todo: optimize it with api lifecycle

	// to find gv through discovery client
	groupList, err := cluster.Client.Discovery().ServerGroups()
	if err != nil {
		return false, nil, err
	}
	if groupList != nil {
		for _, group := range groupList.Groups {
			if group.Name == gvk.Group {
				// found object group in target cluster
				for _, ver := range group.Versions {
					if ver.GroupVersion == gvk.GroupVersion().String() {
						// found match group/version in target cluster.
						// so the object is available in target cluster.
						return true, nil, nil
					}
				}
				// group version in target cluster not match object group version.
				// example: apps/v1beta1 <--> apps/v1
				// we fetch preferred version in that group.
				return false, &schema.GroupVersionKind{Group: group.Name, Version: group.PreferredVersion.Version, Kind: gvk.Kind}, nil
			}
		}
	}

	// going here means the groups about object kind may be different.
	// example: extensions/v1beta1 <--> networking/v1
	// so we should fetch preferred version by object kind.
	_, allResources, err := cluster.Client.Discovery().ServerGroupsAndResources()
	if err != nil {
		return false, nil, err
	}
	if allResources != nil {
		for _, gvs := range allResources {
			for _, resource := range gvs.APIResources {
				if resource.Kind == gvk.Kind {
					// found object kind in target cluster.
					// Attention: if we had crd which kind is same with k8s kind
					// might cause problem, example: foo/bar.pod <--> apps/v1.pod
					return false, &schema.GroupVersionKind{Group: resource.Group, Version: resource.Version, Kind: gvk.Kind}, nil
				}
			}
		}
	}

	notSupportError := fmt.Errorf("%v is not support in target cluster %v with version %v", gvk.String(), targetCluster, clusterVersion)

	return false, nil, notSupportError
}

// Encode encodes given obj
func (c *VersionConverter) Encode(obj runtime.Object, gv runtime.GroupVersioner) ([]byte, error) {
	info, ok := runtime.SerializerInfoForMediaType(c.CF.SupportedMediaTypes(), runtime.ContentTypeJSON)
	if !ok {
		return nil, errors.New("no media type match for serializer")
	}
	encoder := info.Serializer
	codec := c.CF.EncoderForVersion(encoder, gv)
	out, err := runtime.Encode(codec, obj)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Decode decodes data to object
func (c *VersionConverter) Decode(data []byte, defaults *schema.GroupVersionKind, into runtime.Object, versions ...schema.GroupVersion) (runtime.Object, *schema.GroupVersionKind, error) {
	decoder := c.CF.UniversalDecoder(versions...)
	return decoder.Decode(data, defaults, into)
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

func IsStableVersion(gv schema.GroupVersion) bool {
	// todo: is that all stable version
	stableVersions := []string{"v1", "v2", "v3", "v4"}
	for _, v := range stableVersions {
		if v == gv.Version {
			return true
		}
	}
	return false
}

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

type apiLifecycleDeprecated interface {
	APILifecycleDeprecated() (major, minor int)
}

type apiLifecycleRemoved interface {
	APILifecycleRemoved() (major, minor int)
}

type apiLifecycleReplacement interface {
	APILifecycleReplacement() schema.GroupVersionKind
}

type apiLifecycleIntroduced interface {
	APILifecycleIntroduced() (major, minor int)
}
