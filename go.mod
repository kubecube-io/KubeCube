module github.com/kubecube-io/kubecube

go 1.16

require (
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751
	github.com/emicklei/go-restful v2.16.0+incompatible // indirect
	github.com/gin-gonic/gin v1.7.7
	github.com/go-ldap/ldap v3.0.3+incompatible
	github.com/go-logr/logr v1.2.3
	github.com/go-openapi/spec v0.19.5 // indirect
	github.com/go-openapi/swag v0.19.15 // indirect
	github.com/go-playground/validator/v10 v10.5.0 // indirect
	github.com/gogf/gf/v2 v2.0.0-beta
	github.com/golang-jwt/jwt v3.2.1+incompatible
	github.com/google/uuid v1.2.0
	github.com/json-iterator/go v1.1.12
	github.com/leodido/go-urn v1.2.1 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mxk/go-flowrate v0.0.0-20140419014527-cca7078d478f
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/spf13/viper v1.8.1
	github.com/stretchr/testify v1.8.0
	github.com/swaggo/files v0.0.0-20190704085106-630677cd5c14
	github.com/swaggo/gin-swagger v1.3.0
	github.com/swaggo/swag v1.6.7
	github.com/ugorji/go v1.2.5 // indirect
	github.com/urfave/cli/v2 v2.3.0
	go.uber.org/zap v1.19.1
	golang.org/x/net v0.0.0-20220906165146-f3363e06e74c
	gopkg.in/asn1-ber.v1 v1.0.0-20181015200546-f715ec2f112d // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	helm.sh/helm/v3 v3.5.0
	k8s.io/api v0.23.2
	k8s.io/apiextensions-apiserver v0.23.2
	k8s.io/apimachinery v0.23.2
	k8s.io/apiserver v0.20.6
	k8s.io/cli-runtime v0.23.2
	k8s.io/client-go v0.23.2
	k8s.io/klog/v2 v2.30.0
	k8s.io/kubectl v0.20.5
	k8s.io/kubernetes v1.13.0
	k8s.io/metrics v0.20.6
	k8s.io/sample-controller v0.20.4
	rsc.io/letsencrypt v0.0.3 // indirect
	sigs.k8s.io/controller-runtime v0.11.0
	sigs.k8s.io/hierarchical-namespaces v1.0.0
	sigs.k8s.io/yaml v1.3.0
)

replace (
	github.com/containerd/containerd => github.com/containerd/containerd v1.4.11
	// we must controll pkg version manually see issues: https://github.com/kubernetes/client-go/issues/874
	github.com/go-logr/logr => github.com/go-logr/logr v0.4.0
	k8s.io/api => k8s.io/api v0.20.6
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.20.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.20.6
	k8s.io/apiserver => k8s.io/apiserver v0.20.6
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.20.6
	k8s.io/client-go => k8s.io/client-go v0.20.6
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.20.6
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.20.6
	k8s.io/code-generator => k8s.io/code-generator v0.20.6
	k8s.io/component-base => k8s.io/component-base v0.20.6
	k8s.io/component-helpers => k8s.io/component-helpers v0.20.6
	k8s.io/controller-manager => k8s.io/controller-manager v0.20.6
	k8s.io/cri-api => k8s.io/cri-api v0.20.6
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.20.6
	k8s.io/klog/v2 => k8s.io/klog/v2 v2.4.0
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.20.6
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.20.6
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20210305001622-591a79e4bda7
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.20.6
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.20.6
	k8s.io/kubectl => k8s.io/kubectl v0.20.6
	k8s.io/kubelet => k8s.io/kubelet v0.20.6
	k8s.io/kubernetes => k8s.io/kubernetes v1.20.6
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.20.6
	k8s.io/metrics => k8s.io/metrics v0.20.6
	k8s.io/mount-utils => k8s.io/mount-utils v0.20.6
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.20.6
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.8.3
)
