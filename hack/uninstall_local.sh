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

REPO_ROOT=$(git rev-parse --show-toplevel)
cd "${REPO_ROOT}"

kubectl delete -f deploy/manifests/rbac/buildin
kubectl delete -f deploy/manifests/rbac
kubectl delete -f deploy/manifests
kubectl delete -f deploy/metrics-server.yaml
kubectl delete -f deploy/hnc.yaml

make uninstall