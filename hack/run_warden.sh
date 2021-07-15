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

mkdir -p logs

export JWT_SECRET=56F0D8DB90241C6E

go run -mod=vendor cmd/warden/main.go -in-member-cluster=false -tls-cert=deploy/tls/tls.crt -tls-key=deploy/tls/tls.key -webhook-cert=deploy/tls -cluster=pivot-cluster -pivot-cube-host=kubecube.kubecube-system:7443 -log-file=logs/warden.log
