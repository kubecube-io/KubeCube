module github.com/kubecube-io/kubecube

go 1.16

require (
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/gin-gonic/gin v1.7.1
	github.com/go-ldap/ldap v3.0.3+incompatible
	github.com/go-logr/logr v0.4.0
	github.com/go-openapi/swag v0.19.15 // indirect
	github.com/go-playground/validator/v10 v10.5.0 // indirect
	github.com/gogf/gf/v2 v2.0.0-beta
	github.com/google/uuid v1.1.2
	github.com/json-iterator/go v1.1.11
	github.com/leodido/go-urn v1.2.1 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mxk/go-flowrate v0.0.0-20140419014527-cca7078d478f
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.15.0
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/satori/go.uuid v1.2.0
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/spf13/cobra v1.2.1 // indirect
	github.com/spf13/viper v1.8.1
	github.com/stretchr/testify v1.7.0
	github.com/swaggo/files v0.0.0-20190704085106-630677cd5c14
	github.com/swaggo/gin-swagger v1.3.0
	github.com/swaggo/swag v1.6.7
	github.com/ugorji/go v1.2.5 // indirect
	github.com/urfave/cli/v2 v2.3.0
	go.uber.org/zap v1.19.0
	golang.org/x/crypto v0.0.0-20210505212654-3497b51f5e64 // indirect
	golang.org/x/net v0.0.0-20210520170846-37e1c6afe023
	golang.org/x/term v0.0.0-20210220032956-6a3ed077a48d // indirect
	golang.org/x/tools v0.1.5 // indirect
	gopkg.in/asn1-ber.v1 v1.0.0-20181015200546-f715ec2f112d // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	helm.sh/helm/v3 v3.5.0
	k8s.io/api v0.22.1
	k8s.io/apiextensions-apiserver v0.22.1
	k8s.io/apimachinery v0.22.1
	k8s.io/apiserver v0.20.4
	k8s.io/cli-runtime v0.20.5
	k8s.io/client-go v0.22.1
	k8s.io/klog/v2 v2.4.0
	k8s.io/kube-openapi v0.0.0-20210305001622-591a79e4bda7 // indirect
	k8s.io/kubectl v0.20.5
	k8s.io/metrics v0.20.5
	k8s.io/sample-controller v0.20.4
	rsc.io/letsencrypt v0.0.3 // indirect
	sigs.k8s.io/controller-runtime v0.10.0
	sigs.k8s.io/multi-tenancy/incubator/hnc v0.0.0-20210427184539-ae0b4fe06b03
	sigs.k8s.io/structured-merge-diff/v4 v4.1.0 // indirect
	sigs.k8s.io/yaml v1.2.0
)

replace (
	// we must controll pkg version manually see issues: https://github.com/kubernetes/client-go/issues/874
	k8s.io/api => k8s.io/api v0.20.5
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.20.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.20.5
	k8s.io/apiserver => k8s.io/apiserver v0.20.4
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.20.5
	k8s.io/client-go => k8s.io/client-go v0.20.5
	k8s.io/kubectl => k8s.io/kubectl v0.20.5
	k8s.io/metrics => k8s.io/metrics v0.20.5
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.8.3
)
