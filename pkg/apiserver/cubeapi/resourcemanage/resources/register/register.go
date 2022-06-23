package register

import (
	_ "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/cronjob"
	_ "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/deployment"
	_ "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/job"
	_ "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/pod"
	_ "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/pvc"
	_ "github.com/kubecube-io/kubecube/pkg/apiserver/cubeapi/resourcemanage/resources/service"
)
