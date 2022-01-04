#!/usr/bin/env bash

#Copyright 2021 KubeCube Authors
#
#Licensed under the Apache License, Version 2.0 (the "License");
#you may not use this file except in compliance with the License.
#You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
#Unless required by applicable law or agreed to in writing, software
#distributed under the License is distributed on an "AS IS" BASIS,
#WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#See the License for the specific language governing permissions and
#limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

if [[ "$(uname)" == "Darwin" ]]; then
    IPADDR=$(ifconfig | grep inet | grep -v inet6 | grep -v 127 | cut -d ' ' -f2 | sed -n '1p')
    KUBECONFIG=$(cat ~/.kube/config | base64)
elif [[ "$(expr substr $(uname -s) 1 5)" == "Linux" ]]; then
    IPADDR=$(hostname -I |awk '{print $1}')
    KUBECONFIG=$(cat ~/.kube/config | base64 -w 0)
elif [[ "$(expr substr $(uname -s) 1 10)" == "MINGW32_NT" ]]; then
    echo "not support for windows 32"
    exit 1
elif [[ "$(expr substr $(uname -s) 1 10)" == "MINGW64_NT" ]]; then
    echo "not support for windows 64"
fi

K8S_API="$1"

REPO_ROOT=$(git rev-parse --show-toplevel)
cd "${REPO_ROOT}"

function make_manifests() {
  sed s/#K8S_APIEndpoint/${K8S_API}/ deploy/template/pivotCluster.yaml | sed s/#KubeConfig/${KUBECONFIG}/ > deploy/manifests/pivotCluster.yaml
  sed s/#LOCAL_IP/${IPADDR}/ deploy/template/cubeLocalSvc.yaml > deploy/manifests/cubeLocalSvc.yaml
}

make_manifests

make install

kubectl apply -f deploy/manifests/rbac/buildin
kubectl apply -f deploy/manifests/rbac
kubectl apply -f deploy/manifests
kubectl apply -f deploy/metrics-server.yaml
kubectl apply -f deploy/hnc.yaml
