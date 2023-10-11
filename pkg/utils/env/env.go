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

package env

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/multicluster/client/config"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
)

type AuditSvcApi struct {
	URL          string
	Method       string
	Header       string
	AuditHeaders []string
}

func WardenImage() string {
	return os.Getenv("WARDEN_IMAGE")
}

func WardenInitImage() string {
	return os.Getenv("WARDEN_INIT_IMAGE")
}

func DependenceJobImage() string {
	return os.Getenv("DEPENDENCE_JOB_IMAGE")
}

func PivotCubeHost() string {
	return os.Getenv("PIVOT_CUBE_HOST")
}

func WardenRegisterModeEnable() string {
	v := os.Getenv("WARDEN_REGISTER_MODE_ENABLE")
	if v == "" {
		v = "false"
	}
	return v
}

func PivotCubeClusterIPSvc() string {
	r := os.Getenv("PIVOT_CUBE_CLUSTER_IP_SVC")
	if r == "" {
		r = constants.DefaultPivotCubeClusterIPSvc
	}
	return r
}

func AuditIsEnable() bool {
	r := os.Getenv("AUDIT_IS_ENABLE")
	if r == "false" {
		return false
	}
	return true
}

func AuditSVC() AuditSvcApi {
	r := os.Getenv("AUDIT_URL")
	h := os.Getenv("AUDIT_HEADER")
	if r == "" {
		r = constants.DefaultAuditURL
		h = "Content-Type=application/json;charset=UTF-8"
	}
	m := os.Getenv("AUDIT_METHOD")
	if m == "" {
		m = http.MethodPost
	}
	a := strings.Split(os.Getenv("AUDIT_HEADERS"), ",")
	return AuditSvcApi{r, m, h, a}
}

func AuditEventSource() string {
	r := os.Getenv("AUDIT_EVENT_SOURCE")
	if r == "" {
		r = "KubeCube"
	}
	return r
}

func JwtSecret() string {
	return os.Getenv("JWT_SECRET")
}

func ChartsDownload() string {
	r := os.Getenv("DOWNLOAD_CHARTS")
	if r == "" {
		r = "true"
	}
	return r
}

func ChartsDownloadUrl() string {
	return os.Getenv("DOWNLOAD_CHARTS_URL")
}

func AuditLanguage() string {
	l := os.Getenv("AUDIT_LANGUAGE")
	if l == "" {
		l = "en"
	}
	return l
}

func RetainMemberClusterResource() bool {
	res := os.Getenv("RETAIN_MEMBER_CLUSTER_RESOURCE")
	if res == "true" {
		return true
	}

	return false
}

func DetachedNamespaceLabelKey() string {
	return os.Getenv("DETACHED_NS_LABEL_KEY")
}

var (
	once          sync.Once
	cubeNamespace = "kubecube-system"
)

func CubeNamespace() string {
	once.Do(func() {
		ns, ok := os.LookupEnv("CUBE_NAMESPACE")
		if ok {
			cubeNamespace = ns
		}
		clog.Info("kubecube running in namespace %v", cubeNamespace)
	})
	return cubeNamespace
}

// HncManagedLabels is read-only
var HncManagedLabels = hncManagedLabels()

func EnsureManagedLabels(labels map[string]string) map[string]string {
	res := make(map[string]string)
	for k, v := range labels {
		if v != "-" {
			res[k] = v
		}
	}
	return res
}

func hncManagedLabels() map[string]string {
	labels := make(map[string]string)

	labelsStr := os.Getenv("HNC_MANAGED_LABELS")
	if len(labelsStr) == 0 {
		return labels
	}

	// parse labels, format as:
	// add and update: labelKey1@labelValue1;labelKey2@labelValue2
	// delete: labelKey1@-;labelKey2@-
	kvs := strings.Split(labelsStr, ";")
	for _, kv := range kvs {
		res := strings.Split(kv, "@")
		if len(res) != 2 {
			clog.Fatal("labels string invalid: %s", labelsStr)
		}
		labels[res[0]] = res[1]
	}
	return labels
}

func GetClusterClientConfig() config.Config {
	qps := os.Getenv("CLUSTER_CLIENT_QPS")
	qpsFloat, err := strconv.ParseFloat(qps, 32)
	var qpsFloat32 float32
	if err != nil {
		qpsFloat32 = float32(5)
	} else {
		qpsFloat32 = float32(qpsFloat)
	}
	burst := os.Getenv("CLUSTER_CLIENT_BURST")
	burstInt, err := strconv.Atoi(burst)
	if err != nil {
		burstInt = 10
	}
	timeout := os.Getenv("CLUSTER_CLIENT_TIMEOUT_SECONDS")
	timeoutInt, err := strconv.Atoi(timeout)
	if err != nil {
		timeoutInt = 0
	}
	clusterCacheSyncEnable := os.Getenv("DISCOVERY_CACHE_SYNC_ENABLE")
	clusterCacheSyncEnableBool, err := strconv.ParseBool(clusterCacheSyncEnable)
	if err != nil {
		clusterCacheSyncEnableBool = false
	}
	clusterCacheSyncInterval := os.Getenv("DISCOVERY_CACHE_SYNC_PERIOD")
	clusterCacheSyncIntervalInt, err := strconv.Atoi(clusterCacheSyncInterval)
	if err != nil {
		clusterCacheSyncIntervalInt = 60
	}
	return config.Config{
		QPS:                      qpsFloat32,
		Burst:                    burstInt,
		TimeoutSecond:            timeoutInt,
		ClusterCacheSyncEnable:   clusterCacheSyncEnableBool,
		ClusterCacheSyncInterval: clusterCacheSyncIntervalInt,
	}
}
