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

export WARDEN_IMAGE=hub.c.163.com/kubecube/warden:v1.1.0
export DEPENDENCE_JOB_IMAGE=hub.c.163.com/kubecube/warden-dependence:v1.1.0
export WARDEN_INIT_IMAGE=hub.c.163.com/kubecube/warden-init:v1.1.0
export JWT_SECRET=56F0D8DB90241C6E
export PIVOT_CUBE_HOST=kubecube:7443

go run -mod=vendor cmd/cube/main.go -log-level=debug -secure-port=7443 -tls-cert=deploy/tls/tls.crt -tls-key=deploy/tls/tls.key -webhook-cert=deploy/tls -webhook-server-port=9443 -leader-elect=false -log-file=logs/cube.log
